/*
 * Copyright (C) 2014-2017 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package snappy

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/snapcore/snapd/client"
	"github.com/snapcore/snapd/snap"
	. "gopkg.in/check.v1"

	"github.com/snapcore/snapweb/snappy/common"
	"github.com/snapcore/snapweb/snappy/snapdclient"
	"github.com/snapcore/snapweb/statetracker"
)

func Test(t *testing.T) { TestingT(t) }

type GetSnapSuite struct {
	h Handler
	c *snapdclient.FakeSnapdClient
}

var _ = Suite(&GetSnapSuite{})

func (s *GetSnapSuite) SetUpTest(c *C) {
	s.c = &snapdclient.FakeSnapdClient{}
	s.h.setClient(s.c)
}

func (s *GetSnapSuite) TestSnapDoesNotExist(c *C) {
	s.c.Err = errors.New("the snap could not be retrieved locally")
	s.c.StoreErr = errors.New("the snap could not be retrieved from the store")

	_, err := s.h.getSnap("chatroom")
	c.Assert(s.c.Name, Equals, "chatroom")
	c.Assert(err, NotNil)
}

func (s *GetSnapSuite) TestSnapDoesNotExistButStoreHasEmptyResults(c *C) {
	s.c.Err = errors.New("the snap could not be retrieved locally")
	s.c.StoreSnaps = []*client.Snap{}

	_, err := s.h.getSnap("chatroom")
	c.Assert(err, NotNil)
	c.Assert(s.c.Name, Equals, "chatroom")
}

func (s *GetSnapSuite) TestSnapDoesNotExistButStoreHasResults(c *C) {
	s.c.Err = errors.New("the snap could not be retrieved locally")
	s.c.StoreSnaps = []*client.Snap{common.NewSnap("snap1"), common.NewSnap("snap2")}

	_, err := s.h.getSnap("chatroom")
	c.Assert(err, NotNil)
	c.Assert(s.c.Name, Equals, "chatroom")
}

func (s *GetSnapSuite) TestSnapExistsOnStore(c *C) {
	s.c.Err = errors.New("the snap could not be retrieved locally")
	s.c.StoreSnaps = []*client.Snap{common.NewSnap("snap1"), common.NewDefaultSnap(), common.NewSnap("snap2")}

	snap, err := s.h.getSnap("chatroom")
	c.Assert(s.c.Name, Equals, "chatroom")
	c.Assert(err, IsNil)
	c.Assert(snap.Name, Equals, "chatroom")
}

func (s *GetSnapSuite) TestGetSnap(c *C) {
	s.c.Snaps = []*client.Snap{common.NewDefaultSnap()}

	snap, err := s.h.getSnap("chatroom")
	c.Assert(err, IsNil)
	c.Assert(snap.Name, Equals, "chatroom")
}

type PackagePayloadSuite struct {
	h Handler
	c *snapdclient.FakeSnapdClient
}

var _ = Suite(&PackagePayloadSuite{})

func (s *PackagePayloadSuite) SetUpTest(c *C) {
	os.Setenv("SNAP_DATA", c.MkDir())
	s.h.stateTracker = statetracker.New()
	s.c = &snapdclient.FakeSnapdClient{}
	s.h.setClient(s.c)
}

func (s *PackagePayloadSuite) TestPackageNotFound(c *C) {
	s.c.Err = errors.New("the snap could not be retrieved")

	_, err := s.h.packagePayload("chatroom")
	c.Assert(err, NotNil)
}

func (s *PackagePayloadSuite) TestPackage(c *C) {
	s.c.Snaps = []*client.Snap{common.NewDefaultSnap()}

	pkg, err := s.h.packagePayload("chatroom")
	c.Assert(err, IsNil)
	c.Assert(pkg, DeepEquals, snapPkg{
		ID:            "chatroom",
		Description:   "WebRTC Video chat server for Snappy",
		DownloadSize:  0,
		Icon:          "/icons/chatroom_icon.png",
		InstalledSize: 18976651,
		Name:          "chatroom",
		Developer:     "ogra",
		State:         SnapState{Status: "active"},
		Type:          "app",
		Version:       "0.1-8",
		Private:       false,
		InstallDate:   "",
	})
}

type PayloadSuite struct {
	h Handler
}

var _ = Suite(&PayloadSuite{})

func (s *PayloadSuite) SetUpTest(c *C) {
	os.Setenv("SNAP_DATA", c.MkDir())
	s.h.stateTracker = statetracker.New()
	s.h.setClient(&snapdclient.FakeSnapdClient{})
}

func (s *PayloadSuite) TestPayload(c *C) {
	fakeSnap := common.NewDefaultSnap()

	q := s.h.snapToPayload(fakeSnap)

	c.Check(q.Name, Equals, fakeSnap.Name)
	c.Check(q.Version, Equals, fakeSnap.Version)
	c.Check(q.State, DeepEquals, SnapState{Status: statetracker.StatusActive})
	c.Check(q.Type, Equals, snap.Type(fakeSnap.Type))
	c.Check(q.Icon, Equals, "/icons/chatroom_icon.png")
	c.Check(q.Description, Equals, fakeSnap.Description)
}

func (s *PayloadSuite) TestPayloadSnapInstalling(c *C) {
	fakeSnap := common.NewDefaultSnap()
	fakeSnap.Status = client.StatusAvailable
	s.h.stateTracker.TrackInstall("", fakeSnap)

	payload := s.h.snapToPayload(fakeSnap)
	c.Assert(payload.State, DeepEquals, SnapState{Status: statetracker.StatusInstalling})
}

func (s *PayloadSuite) TestPayloadEmptyIconPath(c *C) {
	fakeSnap := common.NewDefaultSnap()

	previousLocalIconPath := localIconPath
	defer func() {
		localIconPath = previousLocalIconPath
	}()

	localIconPath = func(c snapdclient.SnapdClient, name string) (relativePath string, err error) {
		return "", errors.New("error")
	}

	q := s.h.snapToPayload(fakeSnap)
	c.Check(q.Icon, Equals, "")
}

func (s *PayloadSuite) TestPayloadStoreSnapWrongIconPathWithErr(c *C) {
	fakeSnap := common.NewDefaultSnap()
	fakeSnap.Status = client.StatusAvailable
	fakeSnap.Icon = "/v2/icons/chatroom/icon"

	previousLocalIconPath := localIconPath
	defer func() {
		localIconPath = previousLocalIconPath
	}()

	localIconPath = func(c snapdclient.SnapdClient, name string) (relativePath string, err error) {
		return "", errors.New("error")
	}

	q := s.h.snapToPayload(fakeSnap)
	c.Check(q.Icon, Equals, "")
}

func (s *PayloadSuite) TestPayloadStoreSnapWrongIconPathNoErr(c *C) {
	fakeSnap := common.NewDefaultSnap()
	fakeSnap.Status = client.StatusAvailable
	fakeSnap.Icon = "/v2/icons/chatroom/icon"

	previousLocalIconPath := localIconPath
	defer func() {
		localIconPath = previousLocalIconPath
	}()

	localIconPath = func(c snapdclient.SnapdClient, name string) (relativePath string, err error) {
		return "iconpath", nil
	}

	q := s.h.snapToPayload(fakeSnap)
	c.Check(q.Icon, Equals, "iconpath")
}

func (s *PayloadSuite) TestPayloadInstalledSnapBeingRemoved(c *C) {
	fakeSnap := common.NewDefaultSnap()

	previousTryLocateCachedIconForSnap := tryLocateCachedIconForSnap
	previousLocalIconPath := localIconPath
	defer func() {
		localIconPath = previousLocalIconPath
		tryLocateCachedIconForSnap = previousTryLocateCachedIconForSnap
	}()

	localIconPath = func(c snapdclient.SnapdClient, name string) (relativePath string, err error) {
		return "", ErrIconNotExist
	}
	tryLocateCachedIconForSnap = func(snapName string) (string, error) {
		return "", ErrIconNotExist
	}

	q := s.h.snapToPayload(fakeSnap)
	c.Check(q.Icon, Equals, "")

	tryLocateCachedIconForSnap = func(snapName string) (string, error) {
		return "myicon", nil
	}

	q = s.h.snapToPayload(fakeSnap)
	c.Check(q.Icon, Equals, "myicon")
}

type AllPackagesSuite struct {
	c *snapdclient.FakeSnapdClient
	h Handler
}

var _ = Suite(&AllPackagesSuite{})

func (s *AllPackagesSuite) SetUpTest(c *C) {
	os.Setenv("SNAP_DATA", c.MkDir())
	s.h.stateTracker = statetracker.New()
	s.c = &snapdclient.FakeSnapdClient{}
	s.h.setClient(s.c)
}

func (s *AllPackagesSuite) TestNoSnaps(c *C) {
	s.c.StoreErr = errors.New("snaps could not be filtered")

	snaps, err := s.h.allPackages(availableSnaps, "", false, "")
	c.Assert(snaps, IsNil)
	c.Assert(err, NotNil)
}

func (s *AllPackagesSuite) TestPrivateSnaps(c *C) {
	s.c.StoreSnaps = []*client.Snap{}

	_, err := s.h.allPackages(availableSnaps, "", true, "")
	c.Assert(err, IsNil)
	c.Check(s.c.FindOptions.Private, Equals, true)
}

func (s *AllPackagesSuite) TestQueryStringEscaped(c *C) {
	s.c.StoreSnaps = []*client.Snap{}

	_, err := s.h.allPackages(availableSnaps, "de$%**??", true, "")
	c.Assert(err, IsNil)
	c.Check(s.c.FindOptions.Query, Equals, "de%24%25%2A%2A%3F%3F")
}

func (s *AllPackagesSuite) TestHasSnaps(c *C) {
	s.c.StoreSnaps = []*client.Snap{
		common.NewSnap("app2"),
		common.NewSnap("app1"),
	}

	snaps, err := s.h.allPackages(availableSnaps, "", false, "")
	c.Assert(err, IsNil)
	c.Assert(snaps, HasLen, 2)
	// Not sorted, presented as is
	c.Assert(snaps[0].Name, Equals, "app2")
	c.Assert(snaps[1].Name, Equals, "app1")
}

func (s *AllPackagesSuite) TestFormatInstallDate(c *C) {
	c.Assert(formatInstallData(time.Time{}), Equals, "")
	t, _ := time.Parse("2006-Jan-02", "2013-Feb-03")
	c.Assert(formatInstallData(t),
		Equals,
		"Sun Feb  3 00:00:00 UTC 2013")
}

func (s *AllPackagesSuite) TestSnapPrices(c *C) {
	prices := snapPrices{
		"USD": 1.2,
		"EUR": 0.1,
	}

	c.Assert(priceStringFromSnapPrice(snapPrices{}), Equals, "")
	c.Assert(priceStringFromSnapPrice(prices), Equals, "0.1 EUR")
}

type SnapOperationTrackingSuite struct {
	h Handler
	c *snapdclient.FakeSnapdClient
}

var _ = Suite(&SnapOperationTrackingSuite{})

func (s *SnapOperationTrackingSuite) SetUpTest(c *C) {
	os.Setenv("SNAP_DATA", c.MkDir())
	s.c = &snapdclient.FakeSnapdClient{}
	s.h.stateTracker = statetracker.New()
	s.h.setClient(s.c)
}

func (s *SnapOperationTrackingSuite) TestEnableDisableNonLocalSnaps(c *C) {
	fakeSnap := common.NewDefaultSnap()

	fakeSnap.Status = "available"
	s.c.Snaps = []*client.Snap{fakeSnap}

	err := s.h.enable(fakeSnap.Name)
	c.Assert(err, NotNil)

	err = s.h.disable(fakeSnap.Name)
	c.Assert(err, NotNil)
}

func (s *SnapOperationTrackingSuite) TestEnableDisableNonLocalSnapsWithErr(c *C) {
	fakeSnap := common.NewDefaultSnap()

	// Non local snaps

	fakeSnap.Status = "available"
	s.c.Snaps = []*client.Snap{fakeSnap}

	err := s.h.enable(fakeSnap.Name)
	c.Assert(err, NotNil)

	err = s.h.disable(fakeSnap.Name)
	c.Assert(err, NotNil)

	// Various error paths, getSnap error

	fakeSnap.Status = "installed"
	s.c.Err = errors.New("error")

	err = s.h.enable(fakeSnap.Name)
	c.Assert(err, NotNil)

	err = s.h.disable(fakeSnap.Name)
	c.Assert(err, NotNil)

	// Various error paths, getSnap snap not there

	s.c.Snaps = []*client.Snap{}
	s.c.Err = nil

	err = s.h.enable(fakeSnap.Name)
	c.Assert(err, NotNil)

	err = s.h.disable(fakeSnap.Name)
	c.Assert(err, NotNil)
}

func (s *SnapOperationTrackingSuite) TestAbortOperation(c *C) {
	fakeSnap := common.NewDefaultSnap()

	fakeSnap.Status = "available"
	s.c.Snaps = []*client.Snap{fakeSnap}

	s.c.ChangeID = "changeid"
	err := s.h.installPackage(fakeSnap.Name)
	c.Assert(err, IsNil)

	err = s.h.abortRunningOperation(fakeSnap.Name)
	c.Assert(err, IsNil)
	c.Assert(s.c.AbortedChangeID, Equals, "changeid")
}

func (s *SnapOperationTrackingSuite) TestAbortOperationWithErr(c *C) {
	fakeSnap := common.NewDefaultSnap()

	fakeSnap.Status = "available"
	s.c.Snaps = []*client.Snap{}

	err := s.h.abortRunningOperation(fakeSnap.Name)
	c.Assert(err, NotNil)

	s.c.Snaps = []*client.Snap{fakeSnap}
	err = s.h.abortRunningOperation(fakeSnap.Name)
	c.Assert(err, NotNil)
	c.Assert(s.c.AbortedChangeID, Equals, "")
}
