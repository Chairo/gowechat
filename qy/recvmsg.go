// Package qy provides functions for handling the received messages.
package qy

import (
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/bigwhite/gowechat/pb"
)

const (
	// Msg type
	TextMsg     = "text"
	ImageMsg    = "image"
	VoiceMsg    = "voice"
	VideoMsg    = "video"
	LocationMsg = "location"
	EventMsg    = "event"

	// Event type
	SubscribeEvent       = "subscribe"
	UnsubscribeEvent     = "unsubscribe"
	LocationEvent        = "LOCATION"
	MenuClickEvent       = "CLICK"
	MenuViewEvent        = "VIEW"
	ScanCodePushEvent    = "scancode_push"
	ScanCodeWaitEvent    = "scancode_waitmsg"
	PicSysPhotoEvent     = "pic_sysphoto"
	PicPhotoOrAlbumEvent = "pic_photo_or_album"
	PicWeiXinEvent       = "pic_weixin"
	LocationSelectEvent  = "location_select"
	EnterAgentEvent      = "enter_agent"
)

// RecvTextDataPkg is a Text Message received from wechat platform.
type RecvTextDataPkg struct {
	pb.RecvBaseDataPkg
	Content string
	MsgID   uint64
	AgentID int
}

// RecvImageDataPkg is a Image Message received from wechat platform.
type RecvImageDataPkg struct {
	pb.RecvBaseDataPkg
	PicURL  string
	MediaID string
	MsgID   uint64
	AgentID int
}

// RecvVoiceDataPkg is a Voice Message received from wechat platform.
type RecvVoiceDataPkg struct {
	pb.RecvBaseDataPkg
	MediaID string
	Format  string
	MsgID   uint64
	AgentID int
}

// RecvVideoDataPkg is a Video Message received from wechat platform.
type RecvVideoDataPkg struct {
	pb.RecvBaseDataPkg
	MediaID      string
	ThumbMediaID string
	MsgID        uint64
	AgentID      int
}

// RecvLocationDataPkg is a Location Message received from wechat platform.
type RecvLocationDataPkg struct {
	pb.RecvBaseDataPkg
	LocX    float64 `xml:"Location_X"`
	LocY    float64 `xml:"Location_Y"`
	Scale   int
	Label   string
	MsgID   uint64
	AgentID int
}

// RecvSubscribeEventDataPkg is a Subscribe/Unsubscribe event
// Message received from wechat platform.
type RecvSubscribeEventDataPkg struct {
	pb.RecvBaseDataPkg
	Event   string
	AgentID int
}

// RecvLocationEventDataPkg is a Location event Message
// received from wechat platform.
type RecvLocationEventDataPkg struct {
	pb.RecvBaseDataPkg
	Event     string
	Latitude  float64
	Longitude float64
	Precision float64
	AgentID   int
}

// RecvMenuEventDataPkg is a Menu Click event Message
// received from wechat platform.
type RecvMenuEventDataPkg struct {
	pb.RecvBaseDataPkg
	Event    string
	EventKey string
	AgentID  int
}

type recvHandler struct {
	corpID         string
	token          string
	encodingAESKey string
}

// RecvHTTPReqBody is a unmarshall result for below xml data:
// <xml>
// 	<ToUserName><![CDATA[toUser]]</ToUserName>
// 	<AgentID><![CDATA[toAgentID]]</AgentID>
// 	<Encrypt><![CDATA[msg_encrypt]]</Encrypt>
// </xml>
type RecvHTTPReqBody struct {
	ToUserName string
	AgentID    string
	Encrypt    string
}

// RecvHTTPResqBody is a source for marshalling below xml data:
// <xml>
// 	<Encrypt><![CDATA[msg_encrypt]]></Encrypt>
// 	<MsgSignature><![CDATA[msg_signature]]></MsgSignature>
// 	<TimeStamp>timestamp</TimeStamp>
// 	<Nonce><![CDATA[nonce]]></Nonce>
// </xml>
type RecvHTTPRespBody struct {
	Encrypt      pb.CDATAText
	MsgSignature pb.CDATAText
	TimeStamp    int
	Nonce        pb.CDATAText
}

// NewRecvHandler creates an instance of recvHandler
// which implements pb.RecvHandler interface.
func NewRecvHandler(corpID, token, encodingAESKey string) pb.RecvHandler {
	return &recvHandler{corpID: corpID,
		token:          token,
		encodingAESKey: encodingAESKey}
}

// Parse used to parse the receive "post" data request.
// if Parse ok, it return one pkg struct of above; otherwise return error.
//
// Note: We suppose that r.ParseForm() has been invoked before entering this method.
// and we suppose that you have validate the URL in the post request.
func (h *recvHandler) Parse(bodyText []byte) (interface{}, error) {
	var err error

	// XML decoding.
	reqBody := &RecvHTTPReqBody{}
	if err = xml.Unmarshal(bodyText, reqBody); err != nil {
		return nil, err
	}

	fmt.Println(reqBody.Encrypt)
	// Decrpyt the "Encrypt" field.
	var origData []byte
	var corpID string
	origData, _, corpID, err = DecryptMsg(reqBody.Encrypt, h.encodingAESKey)
	if err != nil {
		return nil, err
	}

	fmt.Println(corpID)
	fmt.Println(len(corpID))
	fmt.Println(h.corpID)
	fmt.Println(len(h.corpID))
	if corpID != h.corpID {
		return nil, fmt.Errorf("the request is from corp[%s], not from corp[%s]", corpID, h.corpID)
	}

	fmt.Println(string(origData))

	// Probe the type of message.
	probePkg := &struct {
		MsgType string
		Event   string
	}{}
	if err = xml.Unmarshal(origData, probePkg); err != nil {
		return nil, err
	}

	var dataPkg interface{}
	switch probePkg.MsgType {
	case TextMsg:
		dataPkg = &RecvTextDataPkg{}
	case ImageMsg:
		dataPkg = &RecvImageDataPkg{}
	case VoiceMsg:
		dataPkg = &RecvVoiceDataPkg{}
	case VideoMsg:
		dataPkg = &RecvVideoDataPkg{}
	case LocationMsg:
		dataPkg = &RecvLocationDataPkg{}
	case EventMsg:
		switch probePkg.Event {
		case SubscribeEvent, UnsubscribeEvent:
			dataPkg = &RecvSubscribeEventDataPkg{}
		case LocationEvent:
			dataPkg = &RecvLocationEventDataPkg{}
		case MenuClickEvent, MenuViewEvent:
			dataPkg = &RecvMenuEventDataPkg{}
		case ScanCodePushEvent:
		case ScanCodeWaitEvent:
		case PicSysPhotoEvent:
		case PicPhotoOrAlbumEvent:
		case PicWeiXinEvent:
		case LocationSelectEvent:
		case EnterAgentEvent:
		default:
			return nil, fmt.Errorf("unknown event type: %s", probePkg.Event)
		}

	default:
		return nil, fmt.Errorf("unknown msg type: %s", probePkg.MsgType)
	}

	if err = xml.Unmarshal(origData, dataPkg); err != nil {
		return nil, err
	}
	return dataPkg, nil
}

// ValidateSignature is used to validate the signature in request to figure out
// whether the http request come from wechat qy platform.
func ValidateSignature(signature, token, timestamp, nonce, msgEncrypt string) bool {
	return signature == genSignature(token, timestamp, nonce, msgEncrypt)
}

// dev_msg_signature=sha1(sort(token、timestamp、nonce、msg_encrypt))
func genSignature(token, timestamp, nonce, msgEncrypt string) string {
	sl := []string{token, timestamp, nonce, msgEncrypt}
	sort.Strings(sl)
	s := sha1.New()
	io.WriteString(s, strings.Join(sl, ""))
	return fmt.Sprintf("%x", s.Sum(nil))
}

func ValidateURL(signature, token, timestamp, nonce, cipherEchoStr, encodingAESKey string) (bool, []byte) {
	if !ValidateSignature(signature, token, timestamp, nonce, cipherEchoStr) {
		return false, nil
	}

	echostr, _, _, err := DecryptMsg(cipherEchoStr, encodingAESKey)
	if err != nil {
		return false, nil
	}
	return true, echostr
}