/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.wang@gmicloud.com
 * @Date    : 2025/05/09
 * @Desc    : juicefs 存储类型实现
 */

package storage

import (
	"context"
	"fmt"
	"slices"
	"strings"

	astorage "github.com/karmada-io/karmada/pkg/apis/storage/v1alpha1"
	kcontainerd "github.com/karmada-io/karmada/pkg/containerd"
	"github.com/karmada-io/karmada/pkg/util/cipher"
	"github.com/karmada-io/karmada/pkg/util/exec"
	"github.com/opencontainers/runtime-spec/specs-go"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

type Juicefs struct {
	BaseStorage
	*astorage.Juicefs
	jfsMountConfig *JuiceFSMountConfig
}

// JuiceFSMountConfig 包含挂载 JuiceFS 所需的配置参数
type JuiceFSMountConfig struct {
	MOUNT_POINT                         string
	STORAGE_PATH                        string
	JUICEFS_TOKEN                       string
	JUICEFS_NAME                        string
	MOUNT_OPTIONS                       string
	JUICEFS_CONSOLE_HOST                string
	JUICEFS_META_URL                    string
	JUICEFS_PATH                        string
	JUICEFS_VERSION                     string
	JUICEFS_CACHE_DIR                   string
	JUICEFS_ACCESS_KEY                  string
	JUICEFS_SECRET_KEY                  string
	GCP_APPLICATION_DEFAULT_CREDENTIALS string
	LOG_PATH                            string
}

var (
	MOUNT_OPTIONS = []string{"background", "foreground", "no-syslog", "log", "update-fstab", "max-space", "prefix-internal",
		"hide-internal", "sort-dir", "token", "no-update", "enable-xattr", "enable-acl", "no-bsd-lock", "no-posix-lock",
		"block-interrupt", "readdir-cache", "allow-other", "max-write", "umask", "subdir", "metacacheto", "max-cached-inodes",
		"open-cache", "attr-cache", "entry-cache", "dir-entry-cache", "get-timeout", "put-timeout", "io-retries", "object-clients",
		"max-uploads", "max-downloads", "max-deletes", "flush-wait", "upload-limit", "download-limit", "external", "internal",
		"rsa-key", "flip", "no-sync", "buffer-size", "prefetch", "max-readahead", "initial-readahead", "readahead-ratio",
		"writeback", "cache-dir", "cache-size", "free-space-ratio", "cache-mode", "cache-partial-only", "cache-large-write",
		"verify-cache-checksum", "cache-eviction", "cache-expire", "cache-group", "subgroups", "group-network", "group-ip",
		"group-port", "group-weight", "group-weight-unit", "group-backup", "no-sharing", "fill-group-cache", "cache-group-size",
		"cache-priority", "remote-timeout", "group-compress", "cert-file", "key-file", "gzip", "disallow-list"}
)

func NewJuicefsFromRuntimeObject(ctx context.Context, obj runtime.Object) (*Juicefs, error) {
	subctx := context.WithoutCancel(ctx)
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("failed to convert unstructured to juicefs")
	}
	ajuicefs := &astorage.Juicefs{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, ajuicefs); err != nil {
		return nil, fmt.Errorf("failed to convert unstructured to juicefs: %s", err.Error())
	}
	juicefs := &Juicefs{
		Juicefs: ajuicefs,
		BaseStorage: BaseStorage{
			ctx: subctx,
		},
	}
	// validate the juicefs storage options
	if err := juicefs.Validate(); err != nil {
		return nil, fmt.Errorf("failed to validate juicefs: %s", err.Error())
	}
	return juicefs, nil
}

func (j *Juicefs) Validate() error {
	spec := j.Juicefs.Spec
	// check if the image is from gcp
	switch strings.ToLower(STORAGE_IMAGE_REPO) {
	case "gcp":
		if spec.Auth.GCP == nil {
			return fmt.Errorf("crd define error: auth.gcp or auth.gcp.service-account-credentials is required when juicefs labor repo is gcp")
		}
		credentials, err := cipher.DecryptCompact(spec.Auth.GCP.ServiceAccountCredentials, ENCRYPT_KEY)
		if err != nil {
			return fmt.Errorf("crd define error: failed to decrypt auth.gcp.service-account-credentials: %s", err.Error())
		}
		j.Juicefs.Spec.Auth.GCP.ServiceAccountCredentials = string(credentials)
	case "oss":
		if spec.Auth.OSS == nil {
			return fmt.Errorf("crd define error: auth.oss or auth.oss.access-key or auth.oss.secret-key is required when juicefs labor repo is oss")
		}
		accessKey, err := cipher.DecryptCompact(spec.Auth.OSS.AccessKey, ENCRYPT_KEY)
		if err != nil {
			return fmt.Errorf("crd define error: failed to decrypt auth.oss.access-key: %s", err.Error())
		}
		j.Juicefs.Spec.Auth.OSS.AccessKey = string(accessKey)
		secretKey, err := cipher.DecryptCompact(spec.Auth.OSS.SecretKey, ENCRYPT_KEY)
		if err != nil {
			return fmt.Errorf("crd define error: failed to decrypt auth.oss.secret-key: %s", err.Error())
		}
		j.Juicefs.Spec.Auth.OSS.SecretKey = string(secretKey)
	case "docker":
		if spec.Auth.Docker == nil {
			return fmt.Errorf("crd define error: auth.docker or auth.docker.username or auth.docker.password is required when juicefs labor repo is docker")
		}
		password, err := cipher.DecryptCompact(spec.Auth.Docker.Password, ENCRYPT_KEY)
		if err != nil {
			return fmt.Errorf("crd define error: failed to decrypt auth.docker.password: %s", err.Error())
		}
		j.Juicefs.Spec.Auth.Docker.Password = string(password)
	case "harbor":
		if spec.Auth.Harbor == nil {
			return fmt.Errorf("crd define error: auth.harbor or auth.harbor.username or auth.harbor.password or auth.harbor.url is required when juicefs labor repo is harbor")
		}
		password, err := cipher.DecryptCompact(spec.Auth.Harbor.Password, ENCRYPT_KEY)
		if err != nil {
			return fmt.Errorf("crd define error: failed to decrypt auth.harbor.password: %s", err.Error())
		}
		j.Juicefs.Spec.Auth.Harbor.Password = string(password)
	}

	// check mount-options
	mountOptions := []string{}
	for _, opt := range spec.Client.MountOptions {
		opts := strings.Split(opt, "=")
		// check if the option is valid
		if !slices.Contains(MOUNT_OPTIONS, opts[0]) {
			return fmt.Errorf("crd define error: invalid mount option: %s", opts[0])
		}
		if len(opts) == 1 {
			mountOptions = append(mountOptions, fmt.Sprintf("--%s", opts[0]))
		} else {
			mountOptions = append(mountOptions, fmt.Sprintf("--%s %s", opts[0], strings.Join(opts[1:], "=")))
		}
	}

	j.jfsMountConfig = &JuiceFSMountConfig{
		MOUNT_OPTIONS:     strings.Join(mountOptions, " "),
		MOUNT_POINT:       fmt.Sprintf("%s/%s", MOUNT_POINT, spec.Provider.ID),
		JUICEFS_NAME:      j.Name,
		JUICEFS_CACHE_DIR: j.Spec.Client.CacheDir,
		JUICEFS_PATH:      fmt.Sprintf("%s/juicefs", kcontainerd.STORAGE_PATH),
		STORAGE_PATH:      kcontainerd.STORAGE_PATH,
		LOG_PATH:          fmt.Sprintf("%s/%s.log", kcontainerd.STORAGE_PATH, j.Name),
	}

	if spec.Client.EE != nil {
		if spec.Client.EE.Backend == "" {
			return fmt.Errorf("crd define error: juicefs ee backend is required")
		}
		switch strings.ToLower(spec.Client.EE.Backend) {
		case "gcp":
			if spec.Auth.GCP == nil {
				return fmt.Errorf("crd define error: auth.gcp or auth.gcp.application-default-credentials is required when juicefs ee backend is gcp")
			}
			credentials, err := cipher.DecryptCompact(spec.Auth.GCP.ApplicationDefaultCredentials, ENCRYPT_KEY)
			if err != nil {
				return fmt.Errorf("failed to decrypt application-default-credentials: %s", err.Error())
			}
			j.jfsMountConfig.GCP_APPLICATION_DEFAULT_CREDENTIALS = string(credentials)
		case "oss":
			if spec.Auth.OSS == nil {
				return fmt.Errorf("crd define error: auth.oss or auth.oss.access-key or auth.oss.secret-key is required when juicefs ee backend is oss")
			}
		case "docker":
			if spec.Auth.Docker == nil {
				return fmt.Errorf("crd define error: auth.docker or auth.docker.username or auth.docker.password is required when juicefs ee backend is docker")
			}
		case "harbor":
			if spec.Auth.Harbor == nil {
				return fmt.Errorf("crd define error: auth.harbor or auth.harbor.username or auth.harbor.password or auth.harbor.url is required when juicefs ee backend is harbor")
			}
		default:
			return fmt.Errorf("crd define error: invalid juicefs ee backend: %s", spec.Client.EE.Backend)
		}
		j.jfsMountConfig.JUICEFS_VERSION = j.Spec.Client.EE.Version
		j.jfsMountConfig.JUICEFS_CONSOLE_HOST = j.Spec.Client.EE.ConsoleWeb
		j.jfsMountConfig.JUICEFS_TOKEN = j.Spec.Client.EE.Token
	} else {
		switch strings.ToLower(spec.Client.CE.Backend) {
		case "gcp":
			if spec.Auth.GCP == nil {
				return fmt.Errorf("crd define error: auth.gcp or auth.gcp.application-default-credentials is required when juicefs ce backend is gcp")
			}
		case "oss":
			if spec.Auth.OSS == nil {
				return fmt.Errorf("crd define error: auth.oss or auth.oss.access-key or auth.oss.secret-key is required when juicefs ce backend is oss")
			}
			j.jfsMountConfig.JUICEFS_ACCESS_KEY = j.Spec.Auth.OSS.AccessKey
			j.jfsMountConfig.JUICEFS_SECRET_KEY = j.Spec.Auth.OSS.SecretKey
		case "docker":
			if spec.Auth.Docker == nil {
				return fmt.Errorf("crd define error: auth.docker or auth.docker.username or auth.docker.password is required when juicefs ce backend is docker")
			}
		case "harbor":
			if spec.Auth.Harbor == nil {
				return fmt.Errorf("crd define error: auth.harbor or auth.harbor.username or auth.harbor.password or auth.harbor.url is required when juicefs ce backend is harbor")
			}
		default:
			return fmt.Errorf("crd define error: invalid juicefs ce backend: %s", spec.Client.CE.Backend)
		}
		j.jfsMountConfig.JUICEFS_VERSION = j.Spec.Client.CE.Version
		j.jfsMountConfig.JUICEFS_META_URL = j.Spec.Client.CE.MetaURL
	}

	// decrypt the run script
	runScript, err := cipher.DecryptCompact(spec.Labor.RunScript, ENCRYPT_KEY)
	if err != nil {
		return fmt.Errorf("crd define error: failed to decrypt run script: %s", err.Error())
	}
	j.Juicefs.Spec.Labor.RunScript = string(runScript)

	return nil
}

func (j *Juicefs) Mount() error {
	spec := j.Juicefs.Spec
	// dry run the script
	if err := dryRun(j.ctx, j.Name, string(spec.Labor.RunScript), j.jfsMountConfig); err != nil {
		klog.Errorf("failed to dry run script: %s, please check the the script is correct", err.Error())
		return err
	}
	// write the script file
	if err := writeScript(j.Name, string(spec.Labor.RunScript), j.jfsMountConfig); err != nil {
		klog.Errorf("failed to write script file: %s", err.Error())
		return err
	}
	// create storage path in host
	init := false
	if j.container == nil {
		// create storage path in host
		if err := exec.Command("nsenter", "-t", "1", "-m", "-u", "-n", "-i", "-p", "mkdir", "-p", kcontainerd.STORAGE_PATH).Run(); err != nil {
			klog.Errorf("failed to create storage path: %s", err.Error())
			return err
		}
		j.container = kcontainerd.NewContainer(j.ctx).
			WithNamespace(CONTAINER_NAMESPACE).
			WithName(j.Name).
			WithPrivilege(true).
			WithUser("root")
		init = true
	}
	j.container = j.container.WithMounts(specs.Mount{
		Type:        "bind",
		Source:      kcontainerd.STORAGE_PATH,
		Destination: kcontainerd.STORAGE_PATH,
		Options:     []string{"bind", "rw"},
	}).
		WithImage(STORAGE_IMAGE).
		WithArgs([]string{}).
		WithAuth(&kcontainerd.Auth{
			Username:         "",
			Password:         "",
			InsecureRegistry: false,
			GCPCredentials:   spec.Auth.GCP.ServiceAccountCredentials,
		}).
		WithEnvs(spec.Labor.Envs).
		WithEnvs([]string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			fmt.Sprintf("WATCH_PATH=%s", fmt.Sprintf("%s/%s.sh", kcontainerd.STORAGE_PATH, j.Name)),
			fmt.Sprintf("LOG_PATH=%s", fmt.Sprintf("%s/%s.log", kcontainerd.STORAGE_PATH, j.Name)),
		}).
		WithLogPath(fmt.Sprintf("%s/%s.log", kcontainerd.STORAGE_PATH, j.Name))
	watchContainers <- &WatchContainer{
		Container: j.container,
		Init:      init,
	}
	return nil
}

func (j *Juicefs) Unmount() error {
	klog.Infof("unmounting storage %s", j.Name)
	if err := cc.Delete(j.container); err != nil {
		klog.Errorf("failed to delete containerd container: %s", err.Error())
		return err
	}
	j.container.Cancel()
	klog.Infof("storage %s unmounted", j.Name)
	return nil
}
