package bfacebookbusiness

import (
	"encoding/json"
)

// getRoomID retrieves a matching room ID from the channel name.
func (b *BfacebookBusiness) getRoomID(channel string) string {
	return ""
}

// interface2Struct marshals and immediately unmarshals an interface.
// Useful for converting map[string]interface{} to a struct.
func interface2Struct(in interface{}, out interface{}) error {
	jsonObj, err := json.Marshal(in)
	if err != nil {
		return err //nolint:wrapcheck
	}

	return json.Unmarshal(jsonObj, out)
}

// getDisplayName retrieves the displayName for mxid, querying the homserver if the mxid is not in the cache.
func (b *BfacebookBusiness) getDisplayName(mxid string) string {
	return ""
}

// cacheDisplayName stores the mapping between a mxid and a display name, to be reused later without performing a query to the homserver.
// Note that old entries are cleaned when this function is called.
func (b *BfacebookBusiness) cacheDisplayName(mxid string, displayName string) string {
	return ""
}

// handleError converts errors into httpError.
//nolint:exhaustivestruct

func (b *BfacebookBusiness) containsAttachment(content map[string]interface{}) bool {
	// Skip empty messages
	if content["msgtype"] == nil {
		return false
	}

	// Only allow image,video or file msgtypes
	if !(content["msgtype"].(string) == "m.image" ||
		content["msgtype"].(string) == "m.video" ||
		content["msgtype"].(string) == "m.file") {
		return false
	}

	return true
}

// getAvatarURL returns the avatar URL of the specified sender.
func (b *BfacebookBusiness) getAvatarURL(sender string) string {

	url := ""
	return url
}
