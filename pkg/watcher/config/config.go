package config

import (
	"io"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

var yannotated = `# Handlers know how to send notifications to specific services.
handler:
  slack:
    # Slack "legacy" API token.
    token: ""
    # Slack channel.
    channel: ""
    # Title of the message.
    title: ""
  hipchat:
    # Hipchat token.
    token: ""
    # Room name.
    room: ""
    # URL of the hipchat server.
    url: ""
  mattermost:
    room: ""
    url: ""
    username: ""
  flock:
    # URL of the flock API.
    url: ""
  webhook:
    # Webhook URL.
    url: ""
    # Whether skip tls or not.
    tlsskip: ""
    # Path of webhook cert. Default value is false.
    cert: ""
  cloudevent:
    # CloudEvent webhook URL.
    url: ""
  msteams:
    # MSTeams API Webhook URL.
    webhookurl: ""
  smtp:
    # Destination e-mail address.
    to: ""
    # Sender e-mail address .
    from: ""
    # Smarthost, aka "SMTP server"; address of server used to send email.
    smarthost: ""
    # Subject of the outgoing emails.
    subject: ""
    # Extra e-mail headers to be added to all outgoing messages.
    headers: {}
    # Authentication parameters.
    auth:
      # Username for PLAN and LOGIN auth mechanisms.
      username: ""
      # Password for PLAIN and LOGIN auth mechanisms.
      password: ""
      # Identity for PLAIN auth mechanism
      identity: ""
      # Secret for CRAM-MD5 auth mechanism
      secret: ""
    # If "true" forces secure SMTP protocol (AKA StartTLS).
    requireTLS: false
    # SMTP hello field (optional)
    hello: ""
# Resources to watch.
resource:
  deployment: false
  rc: false
  rs: false
  ds: false
  svc: false
  po: false
  job: false
  node: false
  clusterrole: false
  clusterrolebinding: false
  sa: false
  pv: false
  ns: false
  hpa: false
  secret: false
  configmap: false
  ing: false
# For watching specific namespace, leave it empty for watching all.
# this config is ignored when watching namespaces
namespace: ""
`

var (
	// ConfigFileName stores file of config
	ConfigFileName = ".kubewatch.yaml"

	// ConfigSample is a sample configuration file.
	ConfigSample = yannotated
)

// Handler contains handler configuration
type Handler struct {
	EventBuffer  int          `json:"eventbuffer"`
	Slack        Slack        `json:"slack"`
	SlackWebhook SlackWebhook `json:"slackwebhook"`
	Hipchat      Hipchat      `json:"hipchat"`
	Mattermost   Mattermost   `json:"mattermost"`
	Flock        Flock        `json:"flock"`
	Webhook      Webhook      `json:"webhook"`
	CloudEvent   CloudEvent   `json:"cloudevent"`
	MSTeams      MSTeams      `json:"msteams"`
	SMTP         SMTP         `json:"smtp"`
	Lark         Lark         `json:"lark"`
}

// Resource contains resource configuration
type Resource struct {
	Deployment            bool `json:"deployment"`
	ReplicationController bool `json:"rc"`
	ReplicaSet            bool `json:"rs"`
	DaemonSet             bool `json:"ds"`
	StatefulSet           bool `json:"statefulset"`
	Services              bool `json:"svc"`
	Pod                   bool `json:"po"`
	Job                   bool `json:"job"`
	Node                  bool `json:"node"`
	ClusterRole           bool `json:"clusterrole"`
	ClusterRoleBinding    bool `json:"clusterrolebinding"`
	ServiceAccount        bool `json:"sa"`
	PersistentVolume      bool `json:"pv"`
	Namespace             bool `json:"ns"`
	Secret                bool `json:"secret"`
	ConfigMap             bool `json:"configmap"`
	Ingress               bool `json:"ing"`
	HPA                   bool `json:"hpa"`
	Event                 bool `json:"event"`
	CoreEvent             bool `json:"coreevent"`
}

type CRD struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

type Config struct {
	// Handlers know how to send notifications to specific services.
	Handler Handler `json:"handler"`

	//Reason   []string `json:"reason"`

	// Resources to watch.
	Resource Resource `json:"resource"`

	// CustomResources to Watch
	CustomResources []CRD `json:"customresources"`

	// For watching specific namespace, leave it empty for watching all.
	// this config is ignored when watching namespaces
	Namespace string `json:"namespace,omitempty"`
}

// Slack contains slack configuration
type Slack struct {
	// Slack "legacy" API token.
	Token string `json:"token"`
	// Slack channel.
	Channel string `json:"channel"`
	// Title of the message.
	Title string `json:"title"`
}

// SlackWebhook contains slack configuration
type SlackWebhook struct {
	// Slack channel.
	Channel string `json:"channel"`
	// Slack Username.
	Username string `json:"username"`
	// Slack Emoji.
	Emoji string `json:"emoji"`
	// Slack Webhook Url.
	Slackwebhookurl string `json:"slackwebhookurl"`
}

// Hipchat contains hipchat configuration
type Hipchat struct {
	// Hipchat token.
	Token string `json:"token"`
	// Room name.
	Room string `json:"room"`
	// URL of the hipchat server.
	Url string `json:"url"`
}

// Mattermost contains mattermost configuration
type Mattermost struct {
	Channel  string `json:"room"`
	Url      string `json:"url"`
	Username string `json:"username"`
}

// Flock contains flock configuration
type Flock struct {
	// URL of the flock API.
	Url string `json:"url"`
}

// Webhook contains webhook configuration
type Webhook struct {
	// Webhook URL.
	Url     string `json:"url"`
	Cert    string `json:"cert"`
	TlsSkip bool   `json:"tlsskip"`
}

// Lark contains lark configuration
type Lark struct {
	// Webhook URL.
	WebhookURL string `json:"webhookurl"`
}

// CloudEvent contains CloudEvent configuration
type CloudEvent struct {
	Url string `json:"url"`
}

// MSTeams contains MSTeams configuration
type MSTeams struct {
	// MSTeams API Webhook URL.
	WebhookURL string `json:"webhookurl"`
}

// SMTP contains SMTP configuration.
type SMTP struct {
	// Destination e-mail address.
	To string `json:"to" yaml:"to,omitempty"`
	// Sender e-mail address .
	From string `json:"from" yaml:"from,omitempty"`
	// Smarthost, aka "SMTP server"; address of server used to send email.
	Smarthost string `json:"smarthost" yaml:"smarthost,omitempty"`
	// Subject of the outgoing emails.
	Subject string `json:"subject" yaml:"subject,omitempty"`
	// Extra e-mail headers to be added to all outgoing messages.
	Headers map[string]string `json:"headers" yaml:"headers,omitempty"`
	// Authentication parameters.
	Auth SMTPAuth `json:"auth" yaml:"auth,omitempty"`
	// If "true" forces secure SMTP protocol (AKA StartTLS).
	RequireTLS bool `json:"requireTLS" yaml:"requireTLS"`
	// SMTP hello field (optional)
	Hello string `json:"hello" yaml:"hello,omitempty"`
}

type SMTPAuth struct {
	// Username for PLAN and LOGIN auth mechanisms.
	Username string `json:"username" yaml:"username,omitempty"`
	// Password for PLAIN and LOGIN auth mechanisms.
	Password string `json:"password" yaml:"password,omitempty"`
	// Identity for PLAIN auth mechanism
	Identity string `json:"identity" yaml:"identity,omitempty"`
	// Secret for CRAM-MD5 auth mechanism
	Secret string `json:"secret" yaml:"secret,omitempty"`
}

// New creates new config object
func New() (*Config, error) {
	c := &Config{}
	if err := c.Load(); err != nil {
		return c, err
	}

	return c, nil
}

func createIfNotExist() error {
	// create file if not exist
	configFile := filepath.Join(configDir(), ConfigFileName)
	_, err := os.Stat(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			file, err := os.Create(configFile)
			if err != nil {
				return err
			}
			file.Close()
		} else {
			return err
		}
	}
	return nil
}

// Load loads configuration from config file
func (c *Config) Load() error {
	err := createIfNotExist()
	if err != nil {
		return err
	}

	file, err := os.Open(getConfigFile())
	if err != nil {
		return err
	}

	b, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	if len(b) != 0 {
		return yaml.Unmarshal(b, c)
	}

	return nil
}

// CheckMissingResourceEnvvars will read the environment for equivalent config variables to set
func (c *Config) CheckMissingResourceEnvvars() {
	if !c.Resource.DaemonSet && os.Getenv("KW_DAEMONSET") == "true" {
		c.Resource.DaemonSet = true
	}
	if !c.Resource.ReplicaSet && os.Getenv("KW_REPLICASET") == "true" {
		c.Resource.ReplicaSet = true
	}
	if !c.Resource.Namespace && os.Getenv("KW_NAMESPACE") == "true" {
		c.Resource.Namespace = true
	}
	if !c.Resource.Deployment && os.Getenv("KW_DEPLOYMENT") == "true" {
		c.Resource.Deployment = true
	}
	if !c.Resource.Pod && os.Getenv("KW_POD") == "true" {
		c.Resource.Pod = true
	}
	if !c.Resource.ReplicationController && os.Getenv("KW_REPLICATION_CONTROLLER") == "true" {
		c.Resource.ReplicationController = true
	}
	if !c.Resource.Services && os.Getenv("KW_SERVICE") == "true" {
		c.Resource.Services = true
	}
	if !c.Resource.Job && os.Getenv("KW_JOB") == "true" {
		c.Resource.Job = true
	}
	if !c.Resource.PersistentVolume && os.Getenv("KW_PERSISTENT_VOLUME") == "true" {
		c.Resource.PersistentVolume = true
	}
	if !c.Resource.Secret && os.Getenv("KW_SECRET") == "true" {
		c.Resource.Secret = true
	}
	if !c.Resource.ConfigMap && os.Getenv("KW_CONFIGMAP") == "true" {
		c.Resource.ConfigMap = true
	}
	if !c.Resource.Ingress && os.Getenv("KW_INGRESS") == "true" {
		c.Resource.Ingress = true
	}
	if !c.Resource.Node && os.Getenv("KW_NODE") == "true" {
		c.Resource.Node = true
	}
	if !c.Resource.ServiceAccount && os.Getenv("KW_SERVICE_ACCOUNT") == "true" {
		c.Resource.ServiceAccount = true
	}
	if !c.Resource.ClusterRole && os.Getenv("KW_CLUSTER_ROLE") == "true" {
		c.Resource.ClusterRole = true
	}
	if !c.Resource.ClusterRoleBinding && os.Getenv("KW_CLUSTER_ROLE_BINDING") == "true" {
		c.Resource.ClusterRoleBinding = true
	}
	if (c.Handler.Slack.Channel == "") && (os.Getenv("SLACK_CHANNEL") != "") {
		c.Handler.Slack.Channel = os.Getenv("SLACK_CHANNEL")
	}
	if (c.Handler.Slack.Token == "") && (os.Getenv("SLACK_TOKEN") != "") {
		c.Handler.Slack.Token = os.Getenv("SLACK_TOKEN")
	}
	if (c.Handler.SlackWebhook.Slackwebhookurl == "") && (os.Getenv("KW_SLACK_WEBHOOK_URL") != "") {
		c.Handler.SlackWebhook.Slackwebhookurl = os.Getenv("KW_SLACK_WEBHOOK_URL")
	}
}

func (c *Config) Write() error {
	f, err := os.OpenFile(getConfigFile(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2) // compat with old versions of kubewatch
	return enc.Encode(c)
}

func getConfigFile() string {
	configFile := filepath.Join(configDir(), ConfigFileName)
	if _, err := os.Stat(configFile); err == nil {
		return configFile
	}

	return ""
}

func configDir() string {
	if configDir := os.Getenv("KW_CONFIG"); configDir != "" {
		return configDir
	}

	if runtime.GOOS == "windows" {
		home := os.Getenv("USERPROFILE")
		return home
	}
	return os.Getenv("HOME")
	//path := "/etc/kubewatch"
	//if _, err := os.Stat(path); os.IsNotExist(err) {
	//	os.Mkdir(path, 755)
	//}
	//return path
}
