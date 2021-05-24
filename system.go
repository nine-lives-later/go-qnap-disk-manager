package manager

import (
	"encoding/base64"
	"fmt"
)

type loginResponse struct {
	Username   string `xml:"username"`
	Hostname   string `xml:"hostname"`
	IsAdmin    string `xml:"isAdmin"`
	SessionID  string `xml:"authSid"`
	AuthPassed int    `xml:"authPassed"`
}

// Login perform the authentication against the QNAP storage.
// Any existing session will be logged-out, first.
func (s *QnapSession) Login(username, password string) error {
	// make sure to close any existing sessions
	s.Logout()

	// perform login
	var result loginResponse

	res, err := s.conn.NewRequest(). // see https://download.qnap.com/dev/API_QNAP_QTS_Authentication.pdf
						ExpectContentType("application/json").
						SetQueryParam("user", username).
						SetQueryParam("pwd", encodePassword(password)).
						SetQueryParam("remme", "0").
						SetResult(&result).
						Get("cgi-bin/authLogin.cgi")
	if err != nil {
		return fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}
	if result.AuthPassed != 1 {
		return fmt.Errorf("failed to perform request: authentication failed: %v", string(res.Body()))
	}

	s.sessionID = result.SessionID
	s.conn.SetQueryParam("sid", s.sessionID)

	return nil
}

// Logout invalidates the session.
func (s *QnapSession) Logout() error {
	// no logged-in?
	if s.sessionID == "" {
		return nil
	}

	res, err := s.conn.NewRequest().
		Get("cgi-bin/authLogout.cgi")
	if err != nil {
		return fmt.Errorf("failed to perform request: %v", err)
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("failed to perform request: unexpected HTTP status code: %v", res.StatusCode())
	}

	s.sessionID = ""
	s.conn.SetQueryParam("sid", "")

	return nil
}

func encodePassword(pwd string) string {
	return base64.StdEncoding.EncodeToString([]byte(pwd))
}
