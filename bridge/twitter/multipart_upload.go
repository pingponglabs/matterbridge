package btwitter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// Creates a new file upload http request with optional extra params
func newfileUploadRequest(uri, name string, r io.Reader) (*http.Request, error) {

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("media", name)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, r)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

type MediaUploadResp struct {
	MediaID          int64                `json:"media_id"`
	MediaIDString    string               `json:"media_id_string"`
	Size             int                  `json:"size"`
	ExpiresAfterSecs int                  `json:"expires_after_secs"`
	Errors           mediaUploadErrorResp `json:"errors"`
	Image            struct {
		ImageType string `json:"image_type"`
		W         int    `json:"w"`
		H         int    `json:"h"`
	} `json:"image"`
}
type mediaUploadErrorResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (b *Btwitter) MediaUpload(r io.Reader, name string) (MediaUploadResp, error) {

	request, err := newfileUploadRequest("https://upload.twitter.com/1.1/media/upload.json", name, r)
	if err != nil {
		return MediaUploadResp{}, err
	}
	resp, err := b.client.Do(request)
	if err != nil {
		return MediaUploadResp{}, err
	}
	defer resp.Body.Close()

	uploadResp := MediaUploadResp{}
	err = json.NewDecoder(resp.Body).Decode(&uploadResp)
	if err != nil {
		return MediaUploadResp{}, err
	}
	if uploadResp.Errors.Message != "" {
		return uploadResp, fmt.Errorf("twitter upload media fail : %s", uploadResp.Errors.Message)
	}
	return uploadResp, nil
}
func (b *Btwitter) MediaDownload(mediaUrl string) ([]byte, error) {
	request, err := http.NewRequest("GET", mediaUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	img, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return img, nil
}
