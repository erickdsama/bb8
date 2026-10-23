package connections

import (
	"net/url"
	"strings"
)

// Google accounts (for the calendar connector). Google is not an MCP server and
// does not let apps register by themselves: the owner creates one OAuth client
// in Google Cloud once (its ID and secret are settings, see Flows.Client) and
// then every person signs in with it, through the same bridge as the others.
const KindGoogle = "google"

// KindGoogleHome is Google Home through Google's MCP server (signs in like KindGoogle).
const KindGoogleHome = "googlehome"

// Google's endpoints; variables so tests can point them at a fake.
var (
	GoogleMeta = ASMeta{
		AuthorizationEndpoint: "https://accounts.google.com/o/oauth2/v2/auth",
		TokenEndpoint:         "https://oauth2.googleapis.com/token",
		UserinfoEndpoint:      "https://openidconnect.googleapis.com/v1/userinfo",
	}
	GoogleScope     = "openid email https://www.googleapis.com/auth/calendar.events https://www.googleapis.com/auth/calendar.calendarlist.readonly"
	GoogleHomeScope = "openid email https://www.googleapis.com/auth/home.platform.v2"
	// NestScope is added to a Google sign-in when a Nest Device Access project is
	// set; the sign-in then goes through Nest's partner-connection page, where the
	// person picks which Nest devices to share.
	NestScope = "https://www.googleapis.com/auth/sdm.service"
	NestAuth  = "https://nestservices.google.com/partnerconnections/"
)

// googleAuthURL asks for a refresh token too: offline access, and the consent
// screen every time (Google only sends a refresh token on consent).
func googleAuthURL(a Auth, redirect, state, challenge, scope, nestProject string) string {
	meta := GoogleMeta
	if nestProject != "" {
		scope += " " + NestScope
		meta.AuthorizationEndpoint = NestAuth + url.PathEscape(nestProject) + "/auth"
	}
	u := AuthURL(meta, a, redirect, state, challenge, scope)
	sep := "&"
	if !strings.Contains(u, "?") {
		sep = "?"
	}
	return u + sep + "access_type=offline&prompt=consent"
}

// PickAll lists the accounts that serve a request, oldest first. A person
// reads all of their own accounts; with none of their own, the shared ones.
// known=false is a voice BB-8 doesn't recognize: only shared accounts that
// allow strangers. person "" with known=true is the page, an MCP client or a
// robot without voice ID (pass home.owner); admin (an MCP client) may use any
// account while no owner is set.
func PickAll(list []Connection, known bool, person string, admin bool) []Connection {
	if !known {
		var out []Connection
		for _, c := range list {
			if c.Owner == "" && c.Strangers {
				out = append(out, c)
			}
		}
		return out
	}
	var own, shared []Connection
	for _, c := range list {
		switch {
		case c.Owns(person):
			own = append(own, c)
		case c.Owner == "":
			shared = append(shared, c)
		}
	}
	if len(own) > 0 {
		return own
	}
	if len(shared) > 0 {
		return shared
	}
	if person == "" && admin {
		return list
	}
	return nil
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
