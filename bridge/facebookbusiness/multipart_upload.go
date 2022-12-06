package bfacebookbusiness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/42wim/matterbridge/bridge/config"
)

type Attachment struct {
	Type    string  `json:"type,omitempty"`
	Payload Payload `json:"payload,omitempty"`
}
type Payload struct {
	URL          string `json:"url,omitempty"`
	IsReusable   bool   `json:"is_reusable,omitempty"`
	AttachmentID string `json:"attachment_id,omitempty"`
}

type MediaUploadResp struct {
	Error MediaUploadError `json:"error"`

	AttachmentID string `json:"attachment_id,omitempty"`
}
type MediaUploadError struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Code      int    `json:"code"`
	FbtraceID string `json:"fbtrace_id"`
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}
func CreateFormImage(w *multipart.Writer, contentType, fieldname, filename string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", contentType)
	return w.CreatePart(h)
}

// Creates a new file upload http request with optional extra params
func newfileUploadRequest(uri, fileName string, message *SendImageJson, r io.Reader) (*http.Request, error) {

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	fileDataName := fileName

	part, err := CreateFormImage(writer, "image/jpeg", "filedata", fileDataName)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, r)
	if err != nil {
		return nil, err
	}
	if message != nil {
		msgField, err := json.Marshal(message)
		if err != nil {
			return nil, err
		}
		err = writer.WriteField("message", string(msgField))
		if err != nil {
			return nil, err
		}
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

func (b *BfacebookBusiness) MediaUpload(r io.Reader, name string) (MediaUploadResp, error) {
	Attach := &SendImageJson{Attachment{
		Type: "image",
		Payload: Payload{
			IsReusable: true,
		},
	},
	}
	request, err := newfileUploadRequest("https://graph.facebook.com/v2.10/me/message_attachments", name, Attach, r)
	if err != nil {
		return MediaUploadResp{}, err
	}
	q := request.URL.Query()
	q.Add("access_token", b.Accounts[0].pageAccessToken)
	request.URL.RawQuery = q.Encode()
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return MediaUploadResp{}, err
	}
	defer resp.Body.Close()

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		return MediaUploadResp{}, err
	}
	log.Println(string(bb))
	uploadResp := MediaUploadResp{}
	err = json.Unmarshal(bb, &uploadResp)
	if err != nil {
		return MediaUploadResp{}, err
	}
	if uploadResp.AttachmentID == "" && uploadResp.Error.Message != "" {
		return uploadResp, fmt.Errorf(uploadResp.Error.Message)
	}

	return uploadResp, nil
}
func (b *BfacebookBusiness) HandleInstaMediaUpload(msg *config.Message, recipientID string) (string, error) {
	if msg.Extra == nil {
		return "", fmt.Errorf("nil extra map")
	}

	for _, f := range msg.Extra["file"] {
		if fi, ok := f.(config.FileInfo); ok {
			content := bytes.NewReader(*fi.Data)
			//mtype := mime.TypeByExtension("." + sp[len(sp)-1])

			resp, err := b.InstaMediaUpload(content, fi.Name, recipientID)
			if err != nil {
				return resp, err
			}

			b.Log.Debug(resp)
			return resp, nil
		}
	}
	return "", nil
}

func (b *BfacebookBusiness) InstaMediaUpload(r io.Reader, name, recipientID string) (string, error) {
	fbUrl := fmt.Sprintf("https://graph.facebook.com/v14.0/%s/messages", b.Accounts[0].pageID)
	RecipientParams := &SendRecipientJson{ID: recipientID}
	SendImageParam := &SendImageJson{
		Attachment: Attachment{
			Type: "image",
		},
	}
	recpt, err := json.Marshal(RecipientParams)
	if err != nil {
		return "", err
	}
	message, err := json.Marshal(SendImageParam)
	if err != nil {
		return "", err
	}
	request, err := newfileUploadRequest(fbUrl, name, nil, r)
	if err != nil {
		return "", err
	}
	q := request.URL.Query()

	q.Add("recipient", string(recpt))
	q.Add("message", string(message))

	q.Add("access_token", b.Accounts[0].pageAccessToken)
	request.URL.RawQuery = q.Encode()
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	res := map[string]interface{}{}
	err = json.Unmarshal(bb, &res)
	if err != nil {
		return "", err
	}
	if v, ok := res["message_id"]; ok {
		if msgId, ok := v.(string); ok {
			return msgId, nil
		}
	}
	log.Println(res)
	return "", fmt.Errorf("empty facebook send messageId")
}

func (b *BfacebookBusiness) MediaDownload(mediaUrl string) ([]byte, error) {
	request, err := http.NewRequest("GET", mediaUrl, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(request)
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
