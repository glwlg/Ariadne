package nativecapture

// NetworkMiniState contains only the current values displayed by the native
// window. History and process telemetry stay in the Go network monitor.
type NetworkMiniState struct {
	Visible                bool    `json:"visible"`
	X                      int     `json:"x"`
	Y                      int     `json:"y"`
	Width                  int     `json:"width"`
	Height                 int     `json:"height"`
	UploadBytesPerSecond   float64 `json:"uploadBytesPerSecond"`
	DownloadBytesPerSecond float64 `json:"downloadBytesPerSecond"`
	AdapterName            string  `json:"adapterName"`
	Error                  string  `json:"error,omitempty"`
}

type NetworkMiniReply struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Action  string `json:"action,omitempty"`
}
