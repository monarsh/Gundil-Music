package yt

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// When YouTube answers "Sign in to confirm you're not a bot", it usually
// keeps asking for a while for every song from this server's address.
// Rather than trying each song the usual way first (and failing, slowly),
// streams go straight to a linked YouTube Music account until checkFor has
// passed; then the usual way is tried again.

const checkFor = time.Hour

// SignInCheck describes YouTube's sign-in check as this server last met it.
type SignInCheck struct {
	Active bool      `json:"active"`
	Since  time.Time `json:"since,omitempty"`
	// AccountWorks: songs play signed in with a linked account.
	AccountWorks bool `json:"accountWorks"`
	// AccountFailed: a linked account was tried and YouTube asked again, so
	// its sign-in has most likely expired and it needs linking again.
	AccountFailed bool `json:"accountFailed"`
	// NoAccount: the check came up and no account was linked to play with.
	NoAccount bool `json:"noAccount"`
}

type signIn struct {
	mu       sync.Mutex
	st       SignInCheck
	until    time.Time
	onChange func(SignInCheck)
}

// OnSignInCheck is called (in its own goroutine) when the sign-in check
// starts, ends, or changes how it's being handled.
func (c *Client) OnSignInCheck(f func(SignInCheck)) {
	c.signin.mu.Lock()
	c.signin.onChange = f
	c.signin.mu.Unlock()
}

// SignInCheckState says whether YouTube is asking this server to sign in.
func (c *Client) SignInCheckState() SignInCheck {
	c.signin.mu.Lock()
	defer c.signin.mu.Unlock()
	return c.signin.st
}

func (c *Client) checkActive() bool {
	c.signin.mu.Lock()
	defer c.signin.mu.Unlock()
	return c.signin.st.Active && time.Now().Before(c.signin.until)
}

// noteSignIn records the outcome of a stream lookup. hit: YouTube asked to
// sign in. Then accountWorked / accountFailed / noAccount say what happened
// with the linked accounts. !hit means the usual way worked: the check is over.
func (c *Client) noteSignIn(hit bool, accountWorked, accountFailed, noAccount bool) {
	s := &c.signin
	s.mu.Lock()
	old := s.st
	if !hit {
		s.st = SignInCheck{}
		s.until = time.Time{}
	} else {
		if !s.st.Active {
			s.st = SignInCheck{Active: true, Since: time.Now()}
		}
		s.until = time.Now().Add(checkFor)
		if accountWorked {
			s.st.AccountWorks, s.st.AccountFailed, s.st.NoAccount = true, false, false
		} else {
			if accountFailed {
				s.st.AccountWorks, s.st.AccountFailed = false, true
			}
			s.st.NoAccount = noAccount
		}
	}
	now, f := s.st, s.onChange
	s.mu.Unlock()
	if f != nil && (now.Active != old.Active || now.AccountWorks != old.AccountWorks || now.AccountFailed != old.AccountFailed || now.NoAccount != old.NoAccount) {
		go f(now)
	}
}

// streamSignedIn looks a stream up with each linked account in turn.
func (c *Client) streamSignedIn(ctx context.Context, args map[string]string, files []string) (data json.RawMessage, err error, allAsked bool) {
	allAsked = true
	for _, file := range files {
		a := map[string]string{"cookies": file}
		for k, v := range args {
			a[k] = v
		}
		d, e := c.Call(ctx, "stream", a)
		if e == nil {
			return d, nil, false
		}
		err = e
		if !IsSignInCheck(e) {
			allAsked = false
		}
	}
	return nil, err, allAsked
}
