package dice

import "github.com/sealdice/botgo/dto"

func officialQQPassiveReplyRejected(code int) bool {
	switch code {
	case 304027, // MSG_EXPIRE
		40034005, // msg_id expired
		40034128: // passive reply window or reply count exceeded
		return true
	default:
		return false
	}
}

// sendOfficialQQPacket handles one serialized payload at a time. A report can
// contain multiple media packets: a rejected page must not replay earlier pages.
func sendOfficialQQPacket(
	packet *dto.MessageToCreate,
	allowActiveFallback bool,
	waitActiveQuota func() error,
	post func(*dto.MessageToCreate) (*dto.Message, error),
	noteFallback func(int),
) (*dto.Message, error) {
	if packet.MsgID == "" && packet.EventID == "" {
		if err := waitActiveQuota(); err != nil {
			return nil, err
		}
		return post(packet)
	}

	result, err := post(packet)
	if !allowActiveFallback || result != nil || err == nil {
		return result, err
	}
	code := officialQQAPIErrorCode(err)
	if !officialQQPassiveReplyRejected(code) {
		return result, err
	}

	active := *packet
	active.MsgID, active.EventID, active.MsgSeq = "", "", 0
	active.MessageReference = nil
	if active.Content == "" && active.Media == nil && active.Markdown == nil {
		return result, err
	}
	noteFallback(code)
	if err := waitActiveQuota(); err != nil {
		return nil, err
	}
	return post(&active)
}

func (pa *PlatformAdapterOfficialQQ) logActiveReplyFallback(code int) {
	pa.EndPoint.Session.Parent.Logger.Infof("official qq 被动回复被拒绝（code=%d），对原会话尝试一次主动补发", code)
}
