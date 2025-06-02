/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmicloua.ai
 * @Date    : 2025/05/09
 * @Desc    : 定义存储接口
 */

package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	astorage "github.com/karmada-io/karmada/pkg/apis/storage/v1alpha1"
	kcontainerd "github.com/karmada-io/karmada/pkg/containerd"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/exec"
	"github.com/karmada-io/karmada/pkg/watcher"
	"github.com/karmada-io/karmada/pkg/watcher/config"
	"github.com/karmada-io/karmada/pkg/watcher/handlers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	cc *kcontainerd.ContainerdClient

	watchContainers = make(chan *WatchContainer, 1)

	STORAGE_K8S_NAMESPACE = util.GetEnv("GMI_STORAGE_K8S_NAMESPACE", "gmi-storage")
	CONTAINERD_SOCKET     = util.GetEnv("CONTAINERD_SOCKET", "/run/containerd/containerd.sock")
	MOUNT_POINT           = util.GetEnv("GMI_STORAGE_MOUNT_POINT", "/mnt/juicefs")
	RETRY_COUNT           = util.GetEnv("GMI_STORAGE_RETRY_COUNT", "3")
	CONTAINER_NAMESPACE   = util.GetEnv("GMI_STORAGE_CONTAINER_NAMESPACE", "gmicloud.ai")

	STORAGE_IMAGE      = util.GetEnv("GMI_STORAGE_IMAGE", "us-west1-docker.pkg.dev/devv-404803/public/storage:v0.0.2")
	STORAGE_IMAGE_REPO = util.GetEnv("GMI_STORAGE_IMAGE_REPO", "gcp")
	ENCRYPT_KEY        = util.GetEnv("GMI_STORAGE_ENCRYPT_KEY", "uOvKLmVfztaXGpNYd4Z0I1SiT7MweJhl")
)

type Storage interface {
	Validate() error
	Mount() error
	Unmount() error
}

type WatchContainer struct {
	Container *kcontainerd.Container
	Init      bool
}

type GMIStorage struct {
	ctx              context.Context
	dynamicClientSet dynamic.Interface
	kubeClientSet    kubernetes.Interface
	//TODO: should be a sync.Map
	storages map[string]Storage
}

func NewGMIStorage(ctx context.Context, kubeClientSet kubernetes.Interface, dynamicClientSet dynamic.Interface) *GMIStorage {
	return &GMIStorage{
		ctx:              ctx,
		kubeClientSet:    kubeClientSet,
		dynamicClientSet: dynamicClientSet,
		storages:         map[string]Storage{},
	}
}

func (g *GMIStorage) Exit() {}

func (g *GMIStorage) Init(ctx context.Context, dynamicClientSet dynamic.Interface) error {
	// init containerd client
	if cc == nil {
		nc, err := kcontainerd.NewContainerdClient(CONTAINERD_SOCKET)
		if err != nil {
			klog.Errorf("failed to create containerd client: %s", err.Error())
			return err
		}
		cc = nc
	}
	// fetch all storages
	if err := g.fetchAllStorages(ctx); err != nil {
		return fmt.Errorf("failed to fetch all storages: %s", err.Error())
	}
	for _, storage := range g.storages {
		if err := storage.Mount(); err != nil {
			return fmt.Errorf("failed to mount storage: %s", err.Error())
		}
	}
	return nil
}

func (g *GMIStorage) Watch(ctx context.Context) error {
	conf := config.Config{
		Namespace: STORAGE_K8S_NAMESPACE,
		CustomResources: []config.CRD{
			{
				Group:    astorage.GroupName,
				Version:  astorage.GroupVersion.Version,
				Resource: astorage.ResourcePluralJuicefs,
			},
			// TODO: add more resource types
		},
		Handler: config.Handler{
			EventBuffer: 10,
		},
	}
	handler := g.eventHandler(&conf)
	go watcher.Start(ctx, g.kubeClientSet, g.dynamicClientSet, &conf, handler)

	for {
		select {
		case <-ctx.Done():
			klog.Infof("context done, exit gmi-storage watch")
			return nil
		case container := <-watchContainers:
			klog.Infof("container %s to be watched", container.Container.Name)
			// check if the container image is changed
			old, _ := cc.Get(container.Container.Namespace, container.Container.Name)
			if old != nil && old.Image != container.Container.Image {
				klog.Infof("image of container %s changed from %s to %s, stop old container and create new one", container.Container.Name, old.Image, container.Container.Image)
				if err := cc.Stop(container.Container); err != nil {
					klog.Errorf("failed to stop old container %s: %v, continue to create new container", container.Container.Name, err)
				}
			} else if err := cc.Run(container.Container); err != nil {
				klog.Errorf("failed to run containerd container: %s", err.Error())
				cc.Delete(container.Container)
				return fmt.Errorf("failed to run containerd container: %s", err.Error())
			}
			if container.Init {
				go cc.Watch(container.Container)
				go cc.Logs(container.Container, func(line string) {
					klog.Infof("[%s] %s", container.Container.Name, line)
				})
			}
		case event := <-handler.Events():
			// parse event.obj to storage
			var storageName string
			if event.Kind == astorage.ResourcePluralJuicefs {
				sto, err := NewJuicefsFromRuntimeObject(ctx, event.Obj)
				if err != nil && !IsCrdDefineError(err) {
					return fmt.Errorf("failed to convert unstructured to juicefs: %s", err.Error())
				}
				storageName = sto.Name
				if _, ok := g.storages[sto.Name]; !ok {
					g.storages[sto.Name] = sto
				} else {
					g.storages[sto.Name].(*Juicefs).Juicefs = sto.Juicefs
					g.storages[sto.Name].(*Juicefs).jfsMountConfig = sto.jfsMountConfig
				}
			}
			// TODO: add more storage resource types
			if event.Reason == watcher.EVENT_CREATED && event.Status == watcher.EVENT_NORMAL {
				if err := g.storages[storageName].Mount(); err != nil {
					return fmt.Errorf("failed to mount storage: %s", err.Error())
				}
			} else if event.Reason == watcher.EVENT_UPDATED && event.Status == watcher.EVENT_WARNING {
				if err := g.storages[storageName].Mount(); err != nil {
					return fmt.Errorf("failed to mount storage: %s", err.Error())
				}
			} else if event.Reason == watcher.EVENT_DELETED && event.Status == watcher.EVENT_DANGER {
				if err := g.storages[storageName].Unmount(); err != nil {
					return fmt.Errorf("failed to unmount storage: %s", err.Error())
				}
				delete(g.storages, storageName)
			}
		}
	}
}

// TODO: add more event handlers
func (g *GMIStorage) eventHandler(conf *config.Config) handlers.Handler {
	var eventHandler handlers.Handler
	switch {
	// case len(conf.Handler.Slack.Channel) > 0 || len(conf.Handler.Slack.Token) > 0:
	// 	eventHandler = new(slack.Slack)
	// case len(conf.Handler.SlackWebhook.Channel) > 0 || len(conf.Handler.SlackWebhook.Username) > 0 || len(conf.Handler.SlackWebhook.Slackwebhookurl) > 0:
	// 	eventHandler = new(slackwebhook.SlackWebhook)
	// case len(conf.Handler.Hipchat.Room) > 0 || len(conf.Handler.Hipchat.Token) > 0:
	// 	eventHandler = new(hipchat.Hipchat)
	// case len(conf.Handler.Mattermost.Channel) > 0 || len(conf.Handler.Mattermost.Url) > 0:
	// 	eventHandler = new(mattermost.Mattermost)
	// case len(conf.Handler.Flock.Url) > 0:
	// 	eventHandler = new(flock.Flock)
	// case len(conf.Handler.Webhook.Url) > 0:
	// 	eventHandler = new(webhook.Webhook)
	// case len(conf.Handler.CloudEvent.Url) > 0:
	// 	eventHandler = new(cloudevent.CloudEvent)
	// case len(conf.Handler.MSTeams.WebhookURL) > 0:
	// 	eventHandler = new(msteam.MSTeams)
	// case len(conf.Handler.SMTP.Smarthost) > 0 || len(conf.Handler.SMTP.To) > 0:
	// 	eventHandler = new(smtp.SMTP)
	// case len(conf.Handler.Lark.WebhookURL) > 0:
	// 	eventHandler = new(lark.Webhook)
	default:
		eventHandler = new(handlers.Default)
	}
	if err := eventHandler.Init(conf); err != nil {
		klog.Errorf("failed to init event handler: %s", err.Error())
		panic(fmt.Sprintf("failed to init event handler: %s", err.Error()))
	}
	return eventHandler
}

func (g *GMIStorage) fetchAllStorages(ctx context.Context) error {
	// create resource request object
	gvrs := []schema.GroupVersionResource{
		{
			Group:    astorage.GroupName,             // "storage.karmada.io"
			Version:  astorage.GroupVersion.Version,  // "v1alpha1"
			Resource: astorage.ResourcePluralJuicefs, // "juicefs"
		},
		// TODO: add more resource types
	}
	for _, gvr := range gvrs {
		storageList, err := g.dynamicClientSet.Resource(gvr).Namespace(STORAGE_K8S_NAMESPACE).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list storage: %s", err.Error())
		}
		if storageList == nil {
			return nil
		}
		for _, storage := range storageList.Items {
			if storage.GetKind() == astorage.ResourceKindJuicefs {
				sto, err := NewJuicefsFromRuntimeObject(ctx, &storage)
				if err != nil && !IsCrdDefineError(err) {
					return fmt.Errorf("failed to convert unstructured to juicefs: %s", err.Error())
				}
				if _, ok := g.storages[sto.Name]; !ok {
					g.storages[sto.Name] = sto
				} else {
					g.storages[sto.Name].(*Juicefs).Juicefs = sto.Juicefs
				}
			}
			// TODO: add more resource types
		}
	}
	return nil
}

type StorageStatus string

const (
	StorageStatusInit       StorageStatus = "init"
	StorageStatusMounting   StorageStatus = "mounting"
	StorageStatusMounted    StorageStatus = "mounted"
	StorageStatusUnmounting StorageStatus = "unmounting"
	StorageStatusUnmounted  StorageStatus = "unmounted"
)

type BaseStorage struct {
	Storage
	ctx       context.Context
	container *kcontainerd.Container
}

func writeScript(name, script string, config any) error {
	if _, err := os.Stat(kcontainerd.STORAGE_PATH); os.IsNotExist(err) {
		if err := os.MkdirAll(kcontainerd.STORAGE_PATH, 0755); err != nil {
			return err
		}
	}
	script, err := util.GenerateTemplate(script, name, config)
	if err != nil {
		return err
	}
	scriptFile := fmt.Sprintf("%s/%s.sh", kcontainerd.STORAGE_PATH, name)
	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		return err
	}
	return nil
}

func dryRun(ctx context.Context, name, script string, config any) error {
	script, err := util.GenerateTemplate(script, name, config)
	if err != nil {
		return err
	}
	klog.Infof("dry run script: \n%s", script)
	scriptFile := fmt.Sprintf("/tmp/%s.sh", name)
	if err := os.WriteFile(scriptFile, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write script: %s", err.Error())
	}
	if err := exec.ExecLinuxCmd(ctx, scriptFile, []string{"dry-run"}, "[dry-run] "); err != nil {
		return fmt.Errorf("failed to dry run script: %s", err.Error())
	}
	if err := os.Remove(scriptFile); err != nil {
		return fmt.Errorf("failed to remove script: %s", err.Error())
	}
	return nil
}

func IsCrdDefineError(err error) bool {
	return strings.Contains(err.Error(), "crd define error: ")
}
