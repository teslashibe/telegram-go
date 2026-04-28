package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

// SendMessage sends a plain-text message to the given peer. Honours
// ReplyToID / Silent. Confirm-gated and allowlist-checked.
func (c *Client) SendMessage(ctx context.Context, params SendParams) error {
	if NormalizePeerID(params.PeerID) == "" {
		return fmt.Errorf("%w: PeerID required", ErrInvalidParams)
	}
	if strings.TrimSpace(params.Body) == "" {
		return ErrMessageEmpty
	}
	if err := c.requireConfirm(params.Confirm); err != nil {
		return err
	}
	if err := c.recipientAllowed(params.PeerID); err != nil {
		return err
	}
	if c.dryRun {
		c.logger.Info("dry-run: would SendMessage", logFields(params)...)
		return nil
	}
	return c.withAPI(func(api *tg.Client) error {
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		b := message.NewSender(api).To(ip)
		if params.ReplyToID > 0 {
			b.Reply(params.ReplyToID)
		}
		if params.Silent {
			b.Silent()
		}
		if _, err := b.Text(ctx, params.Body); err != nil {
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		return nil
	})
}

// SendMedia uploads a local file and sends it to the peer with optional
// caption. Kind is auto-detected from MIME or extension when empty.
// Files larger than ~2GB are rejected upstream by Telegram.
func (c *Client) SendMedia(ctx context.Context, params SendMediaParams) error {
	if NormalizePeerID(params.PeerID) == "" {
		return fmt.Errorf("%w: PeerID required", ErrInvalidParams)
	}
	abs, err := filepath.Abs(params.FilePath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidParams, err)
	}
	if st.IsDir() {
		return fmt.Errorf("%w: %s is a directory", ErrInvalidParams, abs)
	}
	if st.Size() == 0 {
		return fmt.Errorf("%w: %s is zero bytes", ErrInvalidParams, abs)
	}
	if err := c.requireConfirm(params.Confirm); err != nil {
		return err
	}
	if err := c.recipientAllowed(params.PeerID); err != nil {
		return err
	}
	kind := params.Kind
	if kind == "" {
		kind = guessMediaKind(abs, params.MIMEType)
	}
	if c.dryRun {
		c.logger.Info("dry-run: would SendMedia", logFields(params)...)
		return nil
	}
	return c.withAPI(func(api *tg.Client) error {
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		up := uploader.NewUploader(api)
		f, err := up.FromPath(ctx, abs)
		if err != nil {
			return fmt.Errorf("%w: upload: %v", ErrSendFailed, err)
		}
		var media message.MediaOption
		switch kind {
		case MediaPhoto:
			media = message.UploadedPhoto(f)
		case MediaSticker:
			media = message.UploadedSticker(f)
		default:
			doc := message.UploadedDocument(f)
			if params.MIMEType != "" {
				doc = doc.MIME(params.MIMEType)
			}
			doc = doc.Filename(filepath.Base(abs))
			media = doc
		}
		b := message.NewSender(api).To(ip)
		if params.Silent {
			b.Silent()
		}
		if _, err := b.Media(ctx, media); err != nil {
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		if params.Caption != "" {
			// Caption sent as a follow-up text. Telegram's media-with-caption
			// API requires StyledTextOption arguments; we keep the surface
			// simple by sending the caption as a separate message in the
			// same context.
			if _, err := message.NewSender(api).To(ip).Text(ctx, params.Caption); err != nil {
				return fmt.Errorf("%w: caption: %v", ErrSendFailed, err)
			}
		}
		return nil
	})
}

// React adds (or removes when Emoji=="") a reaction on the given message.
func (c *Client) React(ctx context.Context, params ReactParams) error {
	if NormalizePeerID(params.PeerID) == "" {
		return fmt.Errorf("%w: PeerID required", ErrInvalidParams)
	}
	if params.MessageID <= 0 {
		return fmt.Errorf("%w: MessageID required", ErrInvalidParams)
	}
	if err := c.requireConfirm(params.Confirm); err != nil {
		return err
	}
	if err := c.recipientAllowed(params.PeerID); err != nil {
		return err
	}
	if c.dryRun {
		c.logger.Info("dry-run: would React", logFields(params)...)
		return nil
	}
	return c.withAPI(func(api *tg.Client) error {
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		var reactions []tg.ReactionClass
		if params.Emoji == "" {
			reactions = []tg.ReactionClass{&tg.ReactionEmpty{}}
		} else {
			reactions = []tg.ReactionClass{&tg.ReactionEmoji{Emoticon: params.Emoji}}
		}
		req := &tg.MessagesSendReactionRequest{
			Peer:     ip,
			MsgID:    params.MessageID,
			Reaction: reactions,
		}
		if params.Big {
			req.Big = true
		}
		if _, err := api.MessagesSendReaction(ctx, req); err != nil {
			return fmt.Errorf("%w: %v", ErrSendFailed, err)
		}
		return nil
	})
}

// JoinChat joins a public group / supergroup / channel by username or
// invite link slug.
func (c *Client) JoinChat(ctx context.Context, params JoinLeaveParams) error {
	if NormalizePeerID(params.PeerID) == "" {
		return fmt.Errorf("%w: PeerID required", ErrInvalidParams)
	}
	if err := c.requireConfirm(params.Confirm); err != nil {
		return err
	}
	if c.dryRun {
		c.logger.Info("dry-run: would JoinChat", logFields(params)...)
		return nil
	}
	return c.withAPI(func(api *tg.Client) error {
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		ch, ok := ip.(*tg.InputPeerChannel)
		if !ok {
			return fmt.Errorf("%w: only channels/supergroups can be joined via this API; %s isn't a channel", ErrInvalidParams, params.PeerID)
		}
		_, err = api.ChannelsJoinChannel(ctx, &tg.InputChannel{
			ChannelID:  ch.ChannelID,
			AccessHash: ch.AccessHash,
		})
		return err
	})
}

// LeaveChat leaves a channel/supergroup.
func (c *Client) LeaveChat(ctx context.Context, params JoinLeaveParams) error {
	if NormalizePeerID(params.PeerID) == "" {
		return fmt.Errorf("%w: PeerID required", ErrInvalidParams)
	}
	if err := c.requireConfirm(params.Confirm); err != nil {
		return err
	}
	if c.dryRun {
		c.logger.Info("dry-run: would LeaveChat", logFields(params)...)
		return nil
	}
	return c.withAPI(func(api *tg.Client) error {
		ip, err := c.inputPeer(ctx, params.PeerID)
		if err != nil {
			return err
		}
		ch, ok := ip.(*tg.InputPeerChannel)
		if !ok {
			return fmt.Errorf("%w: only channels/supergroups can be left via this API", ErrInvalidParams)
		}
		_, err = api.ChannelsLeaveChannel(ctx, &tg.InputChannel{
			ChannelID:  ch.ChannelID,
			AccessHash: ch.AccessHash,
		})
		return err
	})
}

func guessMediaKind(path, mime string) MediaKind {
	mime = strings.ToLower(mime)
	switch {
	case strings.HasPrefix(mime, "image/"):
		return MediaPhoto
	case strings.HasPrefix(mime, "video/"):
		return MediaVideo
	case strings.HasPrefix(mime, "audio/"):
		return MediaAudio
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return MediaPhoto
	case ".mp4", ".mov", ".webm", ".mkv":
		return MediaVideo
	case ".mp3", ".m4a", ".wav", ".ogg", ".flac":
		return MediaAudio
	}
	return MediaDocument
}

var _ = time.Now // future dryRun timestamp logging
