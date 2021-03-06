package kymahelm

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/storage/errors"
	"k8s.io/helm/pkg/tlsutil"
)

// ClientInterface .
type ClientInterface interface {
	ListReleases() (*rls.ListReleasesResponse, error)
	ReleaseStatus(rname string) (string, error)
	IsReleaseDeletable(rname string) (bool, error)
	ReleaseDeployedRevision(rname string) (int32, error)
	InstallReleaseFromChart(chartdir, ns, releaseName, overrides string) (*rls.InstallReleaseResponse, error)
	InstallRelease(chartdir, ns, releasename, overrides string) (*rls.InstallReleaseResponse, error)
	InstallReleaseWithoutWait(chartdir, ns, releasename, overrides string) (*rls.InstallReleaseResponse, error)
	UpgradeRelease(chartDir, releaseName, overrides string) (*rls.UpdateReleaseResponse, error)
	DeleteRelease(releaseName string) (*rls.UninstallReleaseResponse, error)
	PrintRelease(release *release.Release)
	RollbackRelease(releaseName string, revision int32) (*rls.RollbackReleaseResponse, error)
	WaitForReleaseDelete(releaseName string) (bool, error)
	WaitForReleaseRollback(releaseName string) (bool, error)
}

// Client .
type Client struct {
	helm            *helm.Client
	overridesLogger *logrus.Logger
	maxHistory      int32
	timeout         int64
}

// NewClient .
func NewClient(host string, TLSKey string, TLSCert string, TLSInsecureSkipVerify bool, overridesLogger *logrus.Logger, maxHistory int32, timeout int64) (*Client, error) {
	tlsopts := tlsutil.Options{
		KeyFile:            TLSKey,
		CertFile:           TLSCert,
		InsecureSkipVerify: TLSInsecureSkipVerify,
	}
	tlscfg, err := tlsutil.ClientConfig(tlsopts)
	return &Client{
		helm:            helm.NewClient(helm.Host(host), helm.WithTLS(tlscfg), helm.ConnectTimeout(30)),
		overridesLogger: overridesLogger,
		maxHistory:      maxHistory,
		timeout:         timeout,
	}, err
}

// ListReleases lists all releases except for the superseded ones
func (hc *Client) ListReleases() (*rls.ListReleasesResponse, error) {
	statuses := []release.Status_Code{
		release.Status_DELETED,
		release.Status_DELETING,
		release.Status_DEPLOYED,
		release.Status_FAILED,
		release.Status_PENDING_INSTALL,
		release.Status_PENDING_ROLLBACK,
		release.Status_PENDING_UPGRADE,
		release.Status_UNKNOWN,
	}
	return hc.helm.ListReleases(helm.ReleaseListStatuses(statuses))
}

// ReleaseStatus returns roughly-formatted Release status (columns are separated with blanks but not adjusted)
func (hc *Client) ReleaseStatus(rname string) (string, error) {
	status, err := hc.helm.ReleaseStatus(rname)
	if err != nil {
		return "", err
	}
	statusStr := fmt.Sprintf("%+v\n", status)
	return strings.Replace(statusStr, `\n`, "\n", -1), nil
}

//IsReleaseDeletable returns true for release that can be deleted
func (hc *Client) IsReleaseDeletable(rname string) (bool, error) {
	isDeletable := false
	maxAttempts := 3
	fixedDelay := 3

	err := retry.Do(
		func() error {
			statusRes, err := hc.helm.ReleaseStatus(rname)
			if err != nil {
				if strings.Contains(err.Error(), errors.ErrReleaseNotFound(rname).Error()) {
					isDeletable = false
					return nil
				}
				return err
			}
			isDeletable = statusRes.Info.Status.Code != release.Status_DEPLOYED
			return nil
		},
		retry.Attempts(uint(maxAttempts)),
		retry.DelayType(func(attempt uint, config *retry.Config) time.Duration {
			log.Printf("Retry number %d on getting release status.\n", attempt+1)
			return time.Duration(fixedDelay) * time.Second
		}),
	)

	return isDeletable, err
}

func (hc *Client) ReleaseDeployedRevision(rname string) (int32, error) {
	var deployedRevision int32 = 0

	releaseHistoryRes, err := hc.helm.ReleaseHistory(rname, helm.WithMaxHistory(int32(hc.maxHistory)))
	if err != nil {
		return deployedRevision, err
	}

	for _, rel := range releaseHistoryRes.Releases {
		if rel.Info.Status.Code == release.Status_DEPLOYED {
			deployedRevision = rel.Version
		}
	}

	return deployedRevision, nil
}

// InstallReleaseFromChart .
func (hc *Client) InstallReleaseFromChart(chartdir, ns, releaseName, overrides string) (*rls.InstallReleaseResponse, error) {
	chart, err := chartutil.Load(chartdir)

	if err != nil {
		return nil, err
	}

	hc.PrintOverrides(overrides, releaseName, "installation")

	return hc.helm.InstallReleaseFromChart(
		chart,
		ns,
		helm.ReleaseName(string(releaseName)),
		helm.ValueOverrides([]byte(overrides)),
		helm.InstallWait(true),
		helm.InstallTimeout(hc.timeout),
	)
}

// InstallRelease .
func (hc *Client) InstallRelease(chartdir, ns, releasename, overrides string) (*rls.InstallReleaseResponse, error) {
	hc.PrintOverrides(overrides, releasename, "installation")

	return hc.helm.InstallRelease(
		chartdir,
		ns,
		helm.ReleaseName(releasename),
		helm.ValueOverrides([]byte(overrides)),
		helm.InstallWait(true),
		helm.InstallTimeout(hc.timeout),
	)
}

// InstallReleaseWithoutWait .
func (hc *Client) InstallReleaseWithoutWait(chartdir, ns, releasename, overrides string) (*rls.InstallReleaseResponse, error) {
	hc.PrintOverrides(overrides, releasename, "installation")

	return hc.helm.InstallRelease(
		chartdir,
		ns,
		helm.ReleaseName(releasename),
		helm.ValueOverrides([]byte(overrides)),
		helm.InstallWait(false),
		helm.InstallTimeout(hc.timeout),
	)
}

// UpgradeRelease .
func (hc *Client) UpgradeRelease(chartDir, releaseName, overrides string) (*rls.UpdateReleaseResponse, error) {
	hc.PrintOverrides(overrides, releaseName, "update")

	return hc.helm.UpdateRelease(
		releaseName,
		chartDir,
		helm.UpdateValueOverrides([]byte(overrides)),
		helm.ReuseValues(false),
		helm.UpgradeTimeout(hc.timeout),
		helm.UpgradeWait(true),
		helm.UpgradeCleanupOnFail(true),
	)
}

//RollbackRelease performs rollback to given revision
func (hc *Client) RollbackRelease(releaseName string, revision int32) (*rls.RollbackReleaseResponse, error) {
	return hc.helm.RollbackRelease(
		releaseName,
		helm.RollbackWait(true),
		helm.RollbackVersion(revision),
		helm.RollbackCleanupOnFail(true),
		helm.RollbackRecreate(true),
		helm.RollbackTimeout(hc.timeout))
}

// DeleteRelease .
func (hc *Client) DeleteRelease(releaseName string) (*rls.UninstallReleaseResponse, error) {
	return hc.helm.DeleteRelease(
		releaseName,
		helm.DeletePurge(true),
		helm.DeleteTimeout(hc.timeout),
	)
}

//PrintRelease .
func (hc *Client) PrintRelease(release *release.Release) {
	log.Printf("Name: %s", release.Name)
	log.Printf("Namespace: %s", release.Namespace)
	log.Printf("Version: %d", release.Version)
	log.Printf("Status: %s", release.Info.Status.Code)
	log.Printf("Description: %s", release.Info.Description)
}

// PrintOverrides .
func (hc *Client) PrintOverrides(overrides string, releaseName string, action string) {
	hc.overridesLogger.Printf("Overrides used for %s of component %s", action, releaseName)

	if overrides == "" {
		hc.overridesLogger.Println("No overrides found")
		return
	}
	hc.overridesLogger.Println("\n", overrides)
}
