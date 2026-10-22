package connections

import "strings"

// Google accounts (for the calendar connector). Google is not an MCP server and
// does not let apps register by themselves: the owner creates one OAuth client
// in Google Cloud once (its ID and secret are settings, see Flows.Client) and
// then every person signs in with it, through the same bridge as the others.
const KindGoogle = "google"

// Google's endpoints; variables so tests can point them at a fake.
var (
	GoogleMeta = ASMeta{
		AuthorizationEndpoint: "https://accounts.google.com/o/oauth2/v2/auth",
		TokenEndpoint:         "https://oauth2.googleapis.com/token",
		UserinfoEndpoint:      "https://openidconnect.googleapis.com/v1/userinfo",
	}
	GoogleScope = "openid email https://www.googleapis.com/auth/calendar.events"
)

// googleAuthURL asks for a refresh token too: offline access, and the consent
// screen every time (Google only sends a refresh token on consent).
func googleAuthURL(a Auth, redirect, state, challenge string) string {
	u := AuthURL(GoogleMeta, a, redirect, state, challenge, GoogleScope)
	sep := "&"
	if !strings.Contains(u, "?") {
		sep = "?"
	}
	return u + sep + "access_type=offline&prompt=consent"
}

// Pick chooses whose account serves a request: the person's own, else a shared
// one. known=false is a voice BB-8 doesn't recognize: it only gets shared
// accounts that allow strangers, and only to look things up (write=false).
// person "" with known=true is the page, an MCP client or a robot without voice
// ID; pass home.owner then. admin (an MCP client) may use any account while no
// owner is set.
func Pick(list []Connection, known bool, person string, admin, write bool) (Connection, bool) {
	if !known {
		for _, c := range list {
			if c.Owner == "" && c.Strangers && !write {
				return c, true
			}
		}
		return Connection{}, false
	}
	for _, c := range list {
		if c.Owns(person) {
			return c, true
		}
	}
	for _, c := range list {
		if c.Owner == "" {
			return c, true
		}
	}
	if person == "" && admin && len(list) > 0 {
		return list[0], true
	}
	return Connection{}, false
}

// OfKind lists the accounts of one kind, oldest first.
func (s *Store) OfKind(kind string) []Connection {
	var out []Connection
	for _, c := range s.List() {
		if c.Kind == kind {
			out = append(out, c)
		}
	}
	return out
}
