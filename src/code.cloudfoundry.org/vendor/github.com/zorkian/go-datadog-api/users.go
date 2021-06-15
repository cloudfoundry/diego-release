/*
 * Datadog API for Go
 *
 * Please see the included LICENSE file for licensing information.
 *
 * Copyright 2013 by authors and contributors.
 */

package datadog

// reqInviteUsers contains email addresses to send invitations to.
type reqInviteUsers struct {
	Emails []string `json:"emails"`
}

// InviteUsers takes a slice of email addresses and sends invitations to them.
func (self *Client) InviteUsers(emails []string) error {
	return self.doJsonRequest("POST", "/v1/invite_users",
		reqInviteUsers{Emails: emails}, nil)
}
