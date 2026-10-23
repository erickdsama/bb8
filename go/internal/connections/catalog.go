package connections

// Kind is a service the page offers with its own button.
type Kind struct {
	Label  string `json:"label"`
	What   string `json:"what"`
	URL    string `json:"url,omitempty"` // MCP endpoint; empty: the user gives it (or not MCP: google)
	SignIn string `json:"sign_in"`       // oauth, homeassistant, google, auto (probe the link)
	Scope  string `json:"-"`             // google: what the sign-in asks for (default GoogleScope)
	Shared bool   `json:"shared"`        // default owner: the house, not a person
	// Confirm: ask before tools that send, create or delete. Off for the house
	// (lights, plugs): asking "are you sure?" to turn on a lamp gets old fast.
	Confirm bool `json:"confirm"`
	Order   int  `json:"order"`
}

// Catalog: the services with a sign-in button. The MCP ones support the MCP
// authorization flow (OAuth with automatic client registration), checked against
// their published metadata.
var Catalog = map[string]Kind{
	"zapier": {Label: "Zapier", What: "Gmail, Google Calendar, Sheets and thousands of apps.",
		URL: "https://mcp.zapier.com/api/mcp/mcp", SignIn: "oauth", Confirm: true, Order: 1},
	"home": {Label: "Home Assistant", What: "Lights, plugs, thermostats and Alexa devices in your Home Assistant.",
		SignIn: "homeassistant", Shared: true, Order: 2},
	"notion": {Label: "Notion", What: "Your notes, lists and pages.",
		URL: "https://mcp.notion.com/mcp", SignIn: "oauth", Confirm: true, Order: 3},
	"linear": {Label: "Linear", What: "Issues and projects.",
		URL: "https://mcp.linear.app/mcp", SignIn: "oauth", Confirm: true, Order: 4},
	// Google's own MCP server for Google Home (early access; needs a Google Home
	// Premium Advanced subscription). Same Google client as the calendar, enable
	// "Home API" in its Cloud project.
	KindGoogleHome: {Label: "Google Home", What: "Lights, plugs, thermostats and cameras in your Google Home.",
		URL: "https://home.googleapis.com/mcp", SignIn: "google", Scope: GoogleHomeScope, Shared: true, Order: 2},
	// Not an MCP server: the calendar connector uses these accounts (google.go).
	KindGoogle: {Label: "Google Calendar", What: "What's on today, and adding appointments.",
		SignIn: "google", Confirm: true, Order: 5},
	"custom": {Label: "Other", What: "Any service that gives you a link for AI assistants.",
		SignIn: "auto", Confirm: true, Order: 9},
}
