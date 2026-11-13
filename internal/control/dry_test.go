// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"testing"
	"time"

	"github.com/abuzer1495/voltiovolt/internal/dialect"
)

type recordLink struct {
	sends int
}

func (r *recordLink) SendControl(context.Context, Command) (time.Time, error) {
	r.sends++
	return time.Now(), nil
}

func TestDryRunSendsNoControlBytes(t *testing.T) {
	link := &recordLink{}
	c := &Controller{}
	c.SetLink(link, dialect.DialectMegatec)
	c.SetDryRun(true)
	g, err := IPCGrant(true, FuncTest10s, ConfirmStartTest)
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.StartTest(context.Background(), g, FuncTest10s, 0)
	if err != nil || !res.DryRun || !res.Verified || res.Bytes != "T" {
		t.Fatalf("start: %+v %v", res, err)
	}
	if link.sends != 0 {
		t.Fatalf("CONTROL bytes sent: %d", link.sends)
	}
	g, err = IPCGrant(true, FuncCancelTest, "")
	if err != nil {
		t.Fatal(err)
	}
	res, err = c.CancelTest(context.Background(), g)
	if err != nil || !res.DryRun || link.sends != 0 {
		t.Fatalf("cancel: %+v %v sends=%d", res, err, link.sends)
	}
}
