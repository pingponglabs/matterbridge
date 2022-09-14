package bappservice

type AvatarContent struct {
	Info          ImageInfo `json:"info,omitempty"`
	URL           string    `json:"url"`
	ThumbnailURL  string    `json:"thumbnail_url,omitempty"`
	ThumbnailInfo ImageInfo `json:"thumbnail_info,omitempty"`
}

// ImageInfo implements the ImageInfo structure from http://matrix.org/docs/spec/client_server/r0.2.0.html#m-room-avatar
type ImageInfo struct {
	Mimetype string `json:"mimetype"`
	Height   int64  `json:"h"`
	Width    int64  `json:"w"`
	Size     int64  `json:"size"`
}
