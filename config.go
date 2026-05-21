package reverkit

type HTTPServerConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	ListenIP   string `json:"listen_ip" yaml:"listen_ip"`
	ListenPort string `json:"listen_port" yaml:"listen_port"`
	IPHeader   string `json:"ip_header" yaml:"ip_header" #:"在哪个 http header 中取 ip，为空代表从 REMOTE_ADDR 中取"`
}

type ClientConfig struct {
	Token         string `json:"token" yaml:"token" #:"反连平台认证的 Token, 与 Server 保持一致"`
	HTTPBaseURL   string `json:"http_base_url" yaml:"http_base_url" #:"默认将根据 ListenIP 和 ListenPort 生成，该地址是存在漏洞的目标反连回来的地址, 当反连平台前面有反代、绑定域名、端口映射时需要自行配置"`
	RMIServerAddr string `json:"-" yaml:"-" #:"与 http 做端口复用，不用单独配置"`
}

type ServerConfig struct {
	DBFilePath string `json:"db_file_path" yaml:"db_file_path" #:"反连平台数据库文件位置, 这是一个 KV 数据库"`
	Token      string `json:"token" yaml:"token" #:"反连平台认证的 Token, 独立部署时不能为空"`

	HTTPServerConfig HTTPServerConfig `json:"http" yaml:"http"`
}

func (c *ServerConfig) Enabled() bool {
	return c.HTTPServerConfig.Enabled
}

func NewDefaultConfig() *ServerConfig {
	return &ServerConfig{
		DBFilePath:       "",
		HTTPServerConfig: HTTPServerConfig{ListenIP: "0.0.0.0", Enabled: false},
	}
}
