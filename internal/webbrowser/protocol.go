package webbrowser

import "github.com/kejilion/kejilion-panel/internal/browserprotocol"

type Command = browserprotocol.Command
type Event = browserprotocol.Event
type Tab = browserprotocol.Tab

const MaxTabs = browserprotocol.MaxTabs
const MaxCommandBytes = browserprotocol.MaxCommandBytes
const MaxEventBytes = browserprotocol.MaxEventBytes

var DecodeCommand = browserprotocol.DecodeCommand
var WebURL = browserprotocol.WebURL
