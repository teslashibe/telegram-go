# telegram-go

A Go client + MCP tool surface for Telegram (user account, not Bot
API), built on [`github.com/gotd/td`](https://github.com/gotd/td) — the
maintained pure-Go MTProto client.

```go
import "github.com/teslashibe/telegram-go"
```

Same shape as `imessage-go`, `whatsapp-go`, `linkedin-go`, `zillow-go`.
Reads your DMs, groups, channels; sends messages, media, and reactions
on your behalf. **Not** a bot — this needs your phone number, an
`api_id/api_hash` from <https://my.telegram.org/apps>, and a one-time
SMS / 2FA login.

## Architecture

```
┌──────────────────────────────────┐
│ MCP tools (telegram_*)           │  thin wrappers over Client
├──────────────────────────────────┤
│ Client                           │  typed methods (Send, List, …)
│   ├─ session.json                │  gotd auth state (file storage)
│   └─ messages.db (SQLite)        │  local log populated by updates
├──────────────────────────────────┤
│ gotd/td                          │  MTProto transport
└──────────────────────────────────┘
```

Telegram's MTProto API (unlike WhatsApp's) does serve message
**history** on demand, so we can fall back to the live API for
`GetMessages` and `Search` when our local log is sparse. The local log
exists for fast `Watch` polling and offline read patterns.

## Authentication

Get an `api_id` (integer) and `api_hash` (string) from
<https://my.telegram.org/apps> (one-time, free; tied to your account).

```go
client := telegram.New(telegram.Config{
    APIID:    1234567,
    APIHash:  os.Getenv("TELEGRAM_API_HASH"),
    Phone:    "+14155551212",
    StoreDir: "/Users/me/.config/teslashibe/telegram-go",
},
    telegram.WithRequireConfirm(true),
    telegram.WithCodePrompt(func(ctx context.Context) (string, error) {
        // Read the SMS code Telegram just sent.
        return prompt.Read("Telegram SMS code: ")
    }),
    telegram.WithPasswordPrompt(func(ctx context.Context) (string, error) {
        return prompt.ReadPassword("Telegram 2FA password: ")
    }),
)
defer client.Close()

if err := client.Connect(ctx); err != nil { ... }

chats, _ := client.ListChats(ctx, telegram.ChatListParams{Limit: 20})
msgs, _ := client.GetMessages(ctx, telegram.MessageListParams{
    PeerID: chats[0].ID, Limit: 50,
})
_ = client.SendMessage(ctx, telegram.SendParams{
    PeerID:  chats[0].ID,
    Body:    "ack",
    Confirm: true,
})
```

The session is persisted to `<StoreDir>/session.json` (gotd's session
storage) and the local message log to `<StoreDir>/messages.db`.
Subsequent runs skip the SMS/2FA prompts.

## Capability surface

### V1 (read)
| Method | Tool |
|---|---|
| `Status` | `telegram_status` |
| `ListChats` | `telegram_list_chats` |
| `GetMessages` | `telegram_get_messages` |
| `Search` | `telegram_search` |
| `Watch` | `telegram_watch` |
| `ResolveContact` | `telegram_resolve_contact` |

### V1 (write — confirm-gated)
| Method | Tool |
|---|---|
| `SendMessage` | `telegram_send_message` |
| `SendMedia` | `telegram_send_media` |
| `React` | `telegram_react` |
| `MarkRead` | `telegram_mark_read` |
| `JoinChat` / `LeaveChat` | `telegram_join_chat`, `telegram_leave_chat` |

## Safety knobs

- `WithRequireConfirm(true)` (default) — every write tool requires
  `confirm=true` from the agent.
- `WithAllowedPeers([]string)` — sends are denied for off-list peers
  (matched after normalising to canonical peer ID strings).
- `WithDryRun(true)` — write tools log + return without hitting MTProto.

## Drift prevention

`mcp/mcp_test.go` runs `mcptool.Coverage` against `*telegram.Client`.
Any new exported method that isn't wrapped by an MCP tool or listed in
`mcp.Excluded` (with a reason) fails the build.

## Notes

- `api_id/api_hash` are personal credentials — `.gitignore` excludes
  `session.json`, `session.db`, `*.session`, and `.env` files. Pass
  the hash via env var, not source.
- Telegram throttles aggressive flooding; the underlying gotd client
  honours `FLOOD_WAIT_*` errors automatically.
- 2FA users must wire `WithPasswordPrompt`. Without it, login fails
  with `ErrPassword2FARequired`.

## License

MIT (this package). Underlying `gotd/td` is MIT.
