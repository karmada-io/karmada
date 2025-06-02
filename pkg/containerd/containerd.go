/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/05/10
 * @Desc    : containerd 的 client
 */

package containerd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/snapshots"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/opencontainers/runtime-spec/specs-go"
	"k8s.io/klog/v2"
)

var (
	STORAGE_PATH    = util.GetEnv("STORAGE_PATH", "/var/lib/gmi.storage")
	CONTAINERD_PATH = util.GetEnv("CONTAINERD_PATH", "/var/lib/containerd")
)

type Auth struct {
	GCPCredentials   string
	Username         string
	Password         string
	InsecureRegistry bool
}

type Container struct {
	Ctx       context.Context
	Cancel    context.CancelFunc
	Namespace string
	Image     string
	Name      string
	Args      []string
	Envs      []string
	User      string
	Resources *specs.LinuxResources
	Privilege bool
	Status    containerd.ProcessStatus
	LogPath   string
	Mounts    []specs.Mount
	WorkDir   string
	Auth      *Auth
}

func NewContainer(ctx context.Context) *Container {
	sctx, cancel := context.WithCancel(ctx)
	c := &Container{
		Ctx:    sctx,
		Cancel: cancel,
	}
	return c
}

func (c *Container) WithNamespace(namespace string) *Container {
	c.Namespace = namespace
	return c
}

func (c *Container) WithImage(image string) *Container {
	c.Image = image
	return c
}

func (c *Container) WithName(name string) *Container {
	c.Name = name
	return c
}

func (c *Container) WithArgs(args []string) *Container {
	c.Args = args
	return c
}

func (c *Container) WithEnvs(envs []string) *Container {
	c.Envs = envs
	return c
}

func (c *Container) WithUser(user string) *Container {
	c.User = user
	return c
}

func (c *Container) WithResources(resources *specs.LinuxResources) *Container {
	c.Resources = resources
	return c
}

func (c *Container) WithPrivilege(privilege bool) *Container {
	c.Privilege = privilege
	return c
}

func (c *Container) WithLogPath(logPath string) *Container {
	c.LogPath = logPath
	return c
}

func (c *Container) WithAuth(auth *Auth) *Container {
	c.Auth = auth
	return c
}

func (c *Container) WithMounts(mounts ...specs.Mount) *Container {
	c.Mounts = mounts
	return c
}

func (c *Container) WithWorkDir(workDir string) *Container {
	c.WorkDir = workDir
	return c
}

func (c *Container) WithStatus(status containerd.ProcessStatus) *Container {
	c.Status = status
	return c
}

type ContainerdClient struct {
	client *containerd.Client
	socket string
	ctx    context.Context
}

func NewContainerdClient(socketPath string) (*ContainerdClient, error) {
	client, err := containerd.New(socketPath)
	if err != nil {
		return nil, err
	}
	if _, err := client.Version(context.Background()); err != nil {
		return nil, fmt.Errorf("containerd is not running: %w", err)
	}
	return &ContainerdClient{
		client: client,
		socket: socketPath,
		ctx:    context.Background(),
	}, nil
}

// Status check if container exists and is running
func (c *ContainerdClient) Status(container *Container) (containerd.ProcessStatus, error) {
	nsCtx := namespaces.WithNamespace(c.ctx, container.Namespace)

	ctr, err := c.client.LoadContainer(nsCtx, container.Name)
	if err != nil {
		return containerd.Unknown, err
	}

	task, err := ctr.Task(nsCtx, nil)
	if err != nil {
		return containerd.Unknown, err
	}

	status, err := task.Status(nsCtx)
	if err != nil {
		return containerd.Unknown, err
	}

	return status.Status, nil
}

// WithNullIO return a creator that redirect standard I/O to /dev/null
func WithNullIO() cio.Creator {
	return cio.NullIO
}

func (c *ContainerdClient) Run(container *Container) error {
	nsCtx := namespaces.WithNamespace(c.ctx, container.Namespace)
	// check containerd is running
	status, err := c.Status(container)
	container.Status = status
	if err != nil && !errdefs.IsNotFound(err) ||
		(status == containerd.Stopped || status == containerd.Paused || status == containerd.Pausing) {
		klog.Warningf("failed to check container status: %v", err)
		// whether container exists, delete it
		if deleteErr := c.Delete(container); deleteErr != nil {
			klog.Warningf("try to delete old container failed: %v, continue to create new container", deleteErr)
		}
		return fmt.Errorf("failed to check container status: %w", err)
	}
	if status == containerd.Running {
		klog.Infof("container %s already running", container.Name)
		return nil
	}

	// force delete the snapshot
	snapshotName := container.Name + "-snapshot"
	if err := c.cleanupSnapshot(nsCtx, snapshotName); err != nil {
		klog.Warningf("failed to cleanup snapshot %s: %v, continue to create new container", snapshotName, err)
	}

	klog.Infof("pulling image %s", container.Image)
	image, err := c.client.Pull(nsCtx, container.Image, containerd.WithPullUnpack,
		func() containerd.RemoteOpt {
			return containerd.WithResolver(docker.NewResolver(docker.ResolverOptions{
				Credentials: func(host string) (string, string, error) {
					if container.Auth != nil && container.Auth.GCPCredentials != "" &&
						(host == "gcr.io" || host == "us.gcr.io" || host == "eu.gcr.io" ||
							host == "asia.gcr.io" || strings.Contains(host, ".pkg.dev")) {
						return "_json_key", container.Auth.GCPCredentials, nil
					} else if container.Auth != nil {
						return container.Auth.Username, container.Auth.Password, nil
					}
					return "", "", nil
				},
				PlainHTTP: container.Auth != nil && container.Auth.InsecureRegistry,
			}))
		}(),
	)
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	klog.Infof("image %s pulled", container.Image)
	snapshotDir := fmt.Sprintf("%s/custom-snapshots", CONTAINERD_PATH)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// create spec opts
	specOpts := []oci.SpecOpts{
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithImageConfig(image),
		oci.WithEnv(container.Envs),
		oci.WithUser(container.User),
		oci.WithMounts(container.Mounts),
	}

	// add privilege and necessary capabilities
	specOpts = append(specOpts,
		oci.WithPrivileged,
		oci.WithAllCurrentCapabilities,
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithHostDevices,         // mount all host devices
		oci.WithHostHostsFile,       // use host's hosts file
		oci.WithHostResolvconf,      // use host's DNS settings
		oci.WithParentCgroupDevices, // allow access to devices
		oci.WithAllDevicesAllowed,   // allow access to all devices
	)

	if len(container.Args) > 0 {
		specOpts = append(specOpts, oci.WithProcessArgs(container.Args...))
	}

	if container.User != "" {
		specOpts = append(specOpts, oci.WithUser(container.User))
	}

	memoryLimit := oci.WithMemoryLimit(1024 * 1024 * 1024)
	if container.Resources != nil && container.Resources.Memory != nil && container.Resources.Memory.Limit != nil {
		memoryLimit = oci.WithMemoryLimit(uint64(*container.Resources.Memory.Limit))
	}
	specOpts = append(specOpts, memoryLimit)
	cpu := oci.WithCPUs("1")
	if container.Resources != nil && container.Resources.CPU != nil && container.Resources.CPU.Cpus != "" {
		cpu = oci.WithCPUs(container.Resources.CPU.Cpus)
	}
	specOpts = append(specOpts, cpu)

	if container.WorkDir != "" {
		specOpts = append(specOpts, oci.WithProcessCwd(container.WorkDir))
	}

	// create task before creating container
	ioCreator := WithNullIO()
	// set container creation options
	opts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithRuntime("io.containerd.runc.v2", nil),
		containerd.WithNewSnapshot(container.Name+"-snapshot", image,
			snapshots.WithLabels(map[string]string{
				"containerd.io/snapshot.ref": snapshotDir + "/" + container.Name,
			})),
		containerd.WithNewSpec(specOpts...),
	}

	// create container
	ctr, err := c.client.NewContainer(nsCtx, container.Name, opts...)
	if err != nil {
		klog.Errorf("failed to create container in namespace %s: %v", container.Namespace, err)
		return err
	}

	// create task and start it
	task, err := ctr.NewTask(nsCtx, ioCreator)
	if err != nil {
		klog.Errorf("failed to create task in namespace %s: %v", container.Namespace, err)
		return err
	}

	if err := task.Start(nsCtx); err != nil {
		klog.Errorf("failed to start task: %v", err)
		return err
	}
	// check containerd is running
	for range 100 {
		status, err = c.Status(container)
		if err != nil {
			klog.Warningf("failed to check container status: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if status == containerd.Running {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	klog.Infof("container %s started", container.Name)

	return nil
}

func (c *ContainerdClient) Delete(container *Container) error {
	nsCtx := namespaces.WithNamespace(c.ctx, container.Namespace)
	ctr, err := c.client.LoadContainer(nsCtx, container.Name)
	if err != nil {
		return err
	}
	task, err := ctr.Task(nsCtx, nil)
	if err == nil {
		// check task status
		status, err := task.Status(nsCtx)
		if err == nil && status.Status == containerd.Running {
			// container is running, stop it first
			klog.Infof("stopping running container %s", container.Name)
			if err := task.Kill(nsCtx, syscall.SIGTERM); err != nil {
				klog.Warningf("failed to send SIGTERM to container %s: %v, try SIGKILL", container.Name, err)
				if err := task.Kill(nsCtx, syscall.SIGKILL); err != nil {
					klog.Errorf("failed to send SIGKILL to container %s: %v", container.Name, err)
				}
			}

			// wait for container to stop
			ctx, cancel := context.WithTimeout(nsCtx, 20*time.Second)
			defer cancel()

			statusC, err := task.Wait(ctx)
			if err != nil {
				klog.Errorf("failed to wait for container %s to stop: %v", container.Name, err)
			} else {
				select {
				case <-statusC:
					klog.Infof("container %s stopped", container.Name)
				case <-ctx.Done():
					klog.Warningf("waiting for container %s to stop timeout", container.Name)
				}
			}
		}
		// delete task
		exitStatus, err := task.Delete(nsCtx, containerd.WithProcessKill)
		if err != nil {
			klog.Errorf("failed to delete task: %v", err)
			return err
		}
		klog.Infof("task %s deleted, exit code: %v", container.Name, exitStatus.ExitCode())
	}
	if err := ctr.Delete(nsCtx, containerd.WithSnapshotCleanup); err != nil {
		return err
	}
	return nil
}

func (c *ContainerdClient) List(namespace string) (map[string]*Container, error) {
	nsCtx := namespaces.WithNamespace(c.ctx, namespace)
	containers, err := c.client.Containers(nsCtx)
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, nil
	}

	ctrs := make(map[string]*Container)
	for _, container := range containers {
		info, err := container.Info(nsCtx)
		if err != nil {
			return nil, err
		}
		task, err := container.Task(nsCtx, nil)
		if err != nil {
			return nil, err
		}
		status := containerd.Unknown
		statusResponse, err := task.Status(nsCtx)
		if err == nil {
			status = statusResponse.Status
		}
		// TODO: add image
		sctx, cancel := context.WithCancel(c.ctx)
		ctrs[container.ID()] = &Container{
			Ctx:       sctx,
			Cancel:    cancel,
			Namespace: namespace,
			Image:     info.Image,
			Name:      container.ID(),
			Args:      []string{},
			Status:    status,
		}
	}
	return ctrs, nil
}

func (c *ContainerdClient) Get(namespace, name string) (*Container, error) {
	nsCtx := namespaces.WithNamespace(c.ctx, namespace)
	container, err := c.client.LoadContainer(nsCtx, name)
	if err != nil {
		return nil, err
	}
	info, err := container.Info(nsCtx)
	if err != nil {
		return nil, err
	}
	task, err := container.Task(nsCtx, nil)
	if err != nil {
		return nil, err
	}
	status := containerd.Unknown
	statusResponse, err := task.Status(nsCtx)
	if err == nil {
		status = statusResponse.Status
	}
	return &Container{
		Namespace: namespace,
		Image:     info.Image,
		Name:      container.ID(),
		Status:    status,
	}, nil
}

func (c *ContainerdClient) Restart(container *Container) error {
	if err := c.Delete(container); err != nil {
		return err
	}
	return c.Run(container)
}

func (c *ContainerdClient) Stop(container *Container) error {
	nsCtx := namespaces.WithNamespace(c.ctx, container.Namespace)
	ctr, err := c.client.LoadContainer(nsCtx, container.Name)
	if err != nil {
		return err
	}
	task, err := ctr.Task(nsCtx, nil)
	if err != nil {
		return err
	}
	if err := task.Kill(nsCtx, syscall.SIGTERM); err != nil {
		return err
	}
	return nil
}

func (c *ContainerdClient) Watch(container *Container) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			status, err := c.Status(container)
			if (err != nil && !errdefs.IsNotFound(err)) ||
				(status == containerd.Stopped || status == containerd.Paused || status == containerd.Unknown) {
				klog.Warningf("container %s is not running, restarting it", container.Name)
				if err := c.Restart(container); err != nil {
					klog.Errorf("failed to restart container %s: %v", container.Name, err)
				}
			}
		case <-container.Ctx.Done():
			klog.Infof("container %s context done, exit", container.Name)
			return
		}
	}
}

func (c *ContainerdClient) Logs(container *Container, f func(line string)) {
	var (
		logWatcher *util.LogWatcher
		err        error
		logPath    = fmt.Sprintf("%s/%s.log", STORAGE_PATH, container.Name)
	)
	logWatcher, err = util.NewLogWatcher(container.Ctx, logPath, f)
	if err != nil {
		klog.Errorf("failed to create file watcher: %v", err)
		return
	}
	if err := logWatcher.Watch(); err != nil {
		klog.Errorf("failed to watch log: %v", err)
		return
	}
}

// cleanupSnapshot try to remove snapshot
func (c *ContainerdClient) cleanupSnapshot(ctx context.Context, name string) error {
	snapshotter := c.client.SnapshotService("overlayfs")
	if err := snapshotter.Remove(ctx, name); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to remove snapshot %s: %w", name, err)
		}
		// snapshot does not exist, not an error
		return nil
	}
	klog.Infof("snapshot %s removed", name)
	return nil
}
