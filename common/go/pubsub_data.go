package common

type PubSubMessageData struct {
	Bucket   string `json:"bucket"`
	FilePath string `json:"filePath"`
}
