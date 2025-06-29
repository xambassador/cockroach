// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package diagnosticsccl_test

import (
	"context"
	gosql "database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/server"
	"github.com/cockroachdb/cockroach/pkg/server/diagnostics"
	"github.com/cockroachdb/cockroach/pkg/server/diagnostics/diagnosticspb"
	"github.com/cockroachdb/cockroach/pkg/server/telemetry"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/lease"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlstats/persistedsqlstats/sqlstatstestutil"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/diagutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/cloudinfo"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/system"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

const elemName = "somestring"

var setTelemetryHttpTimeout = func(newVal time.Duration) func() {
	prior := diagnostics.TelemetryHttpTimeout
	diagnostics.TelemetryHttpTimeout = newVal
	return func() {
		diagnostics.TelemetryHttpTimeout = prior
	}
}

func TestTenantReport(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	rt := startReporterTest(t, base.TestControlsTenantsExplicitly)
	defer rt.Close()

	tenantArgs := base.TestTenantArgs{
		TenantID:     serverutils.TestTenantID(),
		TestingKnobs: rt.testingKnobs,
	}
	tenant, tenantDB := serverutils.StartTenant(t, rt.server, tenantArgs)
	reporter := tenant.DiagnosticsReporter().(*diagnostics.Reporter)

	ctx := context.Background()
	setupCluster(t, tenantDB)

	// Clear the SQL stat pool before getting diagnostics.
	require.NoError(t, rt.server.SQLServer().(*sql.Server).GetLocalSQLStatsProvider().Reset(ctx))
	reporter.ReportDiagnostics(ctx)

	require.Equal(t, 1, rt.diagServer.NumRequests())

	last := rt.diagServer.LastRequestData()
	lastUUID, err := uuid.FromString(last.UUID)
	require.NoError(t, err)
	require.Equal(t, rt.server.StorageClusterID().ToUint128().Hi, lastUUID.ToUint128().Hi,
		"tenant cluster id should start with storage cluster hi bits")
	require.Equal(t, tenantArgs.TenantID.String(), last.TenantID)
	require.Equal(t, "", last.NodeID)
	require.Equal(t, tenant.SQLInstanceID().String(), last.SQLInstanceID)
	require.Equal(t, "true", last.Internal)

	// Verify environment.
	verifyEnvironment(t, "", roachpb.Locality{}, &last.Env)

	// Verify SQL info.
	require.Equal(t, tenant.SQLInstanceID(), last.SQL.SQLInstanceID)

	// Verify FeatureUsage.
	require.NotZero(t, len(last.FeatureUsage))

	// Call PeriodicallyReportDiagnostics and ensure it sends out a report.
	reporter.PeriodicallyReportDiagnostics(ctx, tenant.AppStopper())
	testutils.SucceedsSoon(t, func() error {
		if rt.diagServer.NumRequests() != 2 {
			return errors.Errorf("did not receive a diagnostics report")
		}
		return nil
	})
}

// TestServerReport checks nodes, stores, localities, and zone configs.
// Telemetry metrics are checked in datadriven tests (see sql.TestTelemetry).
func TestServerReport(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	rt := startReporterTest(t, base.TestIsSpecificToStorageLayerAndNeedsASystemTenant)
	defer rt.Close()

	ctx := context.Background()
	setupCluster(t, rt.serverDB)

	for _, cmd := range []struct {
		resource string
		config   string
	}{
		{"TABLE system.rangelog", fmt.Sprintf(`constraints: [+zone=%[1]s, +%[1]s]`, elemName)},
		{"TABLE system.rangelog", `{gc: {ttlseconds: 1}}`},
		{"DATABASE system", `num_replicas: 5`},
		{"DATABASE system", fmt.Sprintf(`constraints: {"+zone=%[1]s,+%[1]s": 2, +%[1]s: 1}`, elemName)},
		{"DATABASE system", fmt.Sprintf(`experimental_lease_preferences: [[+zone=%[1]s,+%[1]s], [+%[1]s]]`, elemName)},
	} {
		testutils.SucceedsSoon(t, func() error {
			if _, err := rt.serverDB.Exec(
				fmt.Sprintf(`ALTER %s CONFIGURE ZONE = '%s'`, cmd.resource, cmd.config),
			); err != nil {
				// Work around gossip asynchronicity.
				return errors.Wrapf(err, "error applying zone config %q to %q", cmd.config, cmd.resource)
			}
			return nil
		})
	}

	// We want to ensure that non-reportable settings, sensitive
	// settings, and all string settings are redacted. Below we override
	// one of each.
	settingOverrides := []string{
		`SET CLUSTER SETTING server.oidc_authentication.client_id = 'sensitive-client-id'`, // Sensitive setting.
		`SET CLUSTER SETTING sql.log.user_audit = 'test_role NONE'`,                        // Non-reportable setting.
		`SET CLUSTER SETTING changefeed.node_throttle_config = '{"message_rate": 0.5}'`,    // String setting.
	}
	for _, s := range settingOverrides {
		_, err := rt.serverDB.Exec(s)
		require.NoError(t, err)
	}

	// We want to ensure that non-reportable settings, sensitive
	// settings, and all string settings are redacted. Below we override
	// one of each.
	_, err := rt.serverDB.Exec(`SET application_name = 'test'`)
	require.NoError(t, err)
	schemaAndQueriesForTest := []string{
		`CREATE TABLE diagnostics_test_table (diagnostics_test_id int)`,
		`ALTER TABLE diagnostics_test_table ADD COLUMN diagnostics_test_name string`,
		`INSERT INTO diagnostics_test_table VALUES (123456, 'diagnostics_test_name_value') ON CONFLICT DO NOTHING`,
	}
	for _, s := range schemaAndQueriesForTest {
		_, err := rt.serverDB.Exec(s)
		require.NoError(t, err)
	}

	conn := sqlutils.MakeSQLRunner(rt.serverDB)
	sqlstatstestutil.WaitForStatementEntriesAtLeast(t, conn, len(schemaAndQueriesForTest),
		sqlstatstestutil.StatementFilter{App: "test"})

	expectedUsageReports := 0

	clusterSecret := sql.ClusterSecret.Get(&rt.settings.SV)

	foundStmt := false
	testutils.SucceedsSoon(t, func() error {
		expectedUsageReports++

		node := rt.server.MetricsRecorder().GenerateNodeStatus(ctx)
		// Clear the SQL stat pool before getting diagnostics.
		require.NoError(t, rt.server.SQLServer().(*sql.Server).GetLocalSQLStatsProvider().Reset(ctx))
		rt.server.DiagnosticsReporter().(*diagnostics.Reporter).ReportDiagnostics(ctx)

		keyCounts := make(map[roachpb.StoreID]int64)
		rangeCounts := make(map[roachpb.StoreID]int64)
		totalKeys := int64(0)
		totalRanges := int64(0)

		for _, store := range node.StoreStatuses {
			keys, ok := store.Metrics["keycount"]
			require.True(t, ok, "keycount not in metrics")
			totalKeys += int64(keys)
			keyCounts[store.Desc.StoreID] = int64(keys)

			replicas, ok := store.Metrics["replicas"]
			require.True(t, ok, "replicas not in metrics")
			totalRanges += int64(replicas)
			rangeCounts[store.Desc.StoreID] = int64(replicas)
		}

		require.Equal(t, expectedUsageReports, rt.diagServer.NumRequests())

		last := rt.diagServer.LastRequestData()

		// Verify SQL Stats fingerprinting. We need to run this check
		// inside the `SucceedsSoon` call because the diagnostic reporter
		// resets SQL stats once it's completed the report. This means that
		// if this procedure is retried, subsequent values of `last` will
		// be missing the sql stats since they will have been "consumed".
		// Hence, we proactively look through them for the fingerprint.
		if len(last.SqlStats) > 0 {
			for _, s := range last.SqlStats {
				require.False(t, strings.HasPrefix(s.Key.App, "$ internal"), "expected app name %s to not be internal", s.Key.App)
				if s.Key.Query == "INSERT INTO _ VALUES (_, __more__) ON CONFLICT DO NOTHING" {
					foundStmt = true
					require.Equal(t, int64(1), s.Stats.Count)
				}
			}
		}

		if minExpected, actual := totalKeys, last.Node.KeyCount; minExpected > actual {
			return errors.Errorf("expected node keys at least %v got %v", minExpected, actual)
		}
		if minExpected, actual := totalRanges, last.Node.RangeCount; minExpected > actual {
			return errors.Errorf("expected node ranges at least %v got %v", minExpected, actual)
		}
		if minExpected, actual := len(rt.serverArgs.StoreSpecs), len(last.Stores); minExpected > actual {
			return errors.Errorf("expected at least %v stores got %v", minExpected, actual)
		}

		for _, store := range last.Stores {
			if minExpected, actual := keyCounts[store.StoreID], store.KeyCount; minExpected > actual {
				return errors.Errorf("expected at least %v keys in store %v got %v", minExpected, store.StoreID, actual)
			}
			if minExpected, actual := rangeCounts[store.StoreID], store.RangeCount; minExpected > actual {
				return errors.Errorf("expected at least %v ranges in store %v got %v", minExpected, store.StoreID, actual)
			}
		}
		return nil
	})

	require.True(t, foundStmt, "expected to find INSERT INTO _ VALUES (_, __more__) ON CONFLICT DO NOTHING in stats")

	last := rt.diagServer.LastRequestData()
	require.Equal(t, rt.server.StorageClusterID().String(), last.UUID)
	require.Equal(t, "system", last.TenantID)
	require.Equal(t, rt.server.NodeID().String(), last.NodeID)
	require.Equal(t, rt.server.NodeID().String(), last.SQLInstanceID)
	require.Equal(t, "true", last.Internal)

	// Verify environment.
	verifyEnvironment(t, clusterSecret, rt.serverArgs.Locality, &last.Env)

	// This check isn't clean, since the body is a raw proto binary and thus could
	// easily contain some encoded form of elemName, but *if* it ever does fail,
	// that is probably very interesting.
	require.NotContains(t, last.RawReportBody, elemName)

	// 3 + 3 = 6: set 3 initially and org is set mid-test for 3 altered settings,
	// plus version, reporting and secret settings are set in startup
	// migrations.
	expected, actual := 7+len(settingOverrides), len(last.AlteredSettings)
	require.Equal(t, expected, actual, "expected %d changed settings, got %d: %v", expected, actual, last.AlteredSettings)

	for key, expected := range map[string]string{
		// Note: this uses setting _keys_, not setting names.
		"cluster.organization":                     "<redacted>",
		"cluster.label":                            "<redacted>",
		"diagnostics.reporting.send_crash_reports": "false",
		"server.time_until_store_dead":             "1m30s",
		"version":                                  clusterversion.Latest.String(),
		"cluster.secret":                           "<redacted>",
		"server.oidc_authentication.client_id":     "<redacted>",
		"sql.log.user_audit":                       "<redacted>",
		"changefeed.node_throttle_config":          "<redacted>",
	} {
		got, ok := last.AlteredSettings[key]
		require.True(t, ok, "expected report of altered setting %q", key)
		require.Equal(t, expected, got, "expected reported value of setting %q to be %q not %q", key, expected, got)
	}

	// Verify that we receive the four auto-populated zone configs plus the two
	// modified above, and that their values are as expected.
	for _, expectedID := range []int64{
		keys.RootNamespaceID,
		keys.LivenessRangesID,
		keys.MetaRangesID,
		keys.RangeEventTableID,
		keys.SystemDatabaseID,
	} {
		_, ok := last.ZoneConfigs[expectedID]
		require.True(t, ok, "didn't find expected ID %d in reported ZoneConfigs: %+v",
			expectedID, last.ZoneConfigs)
	}
	hashedElemName := sql.HashForReporting(clusterSecret, elemName)
	hashedZone := sql.HashForReporting(clusterSecret, "zone")
	for id, zone := range last.ZoneConfigs {
		if id == keys.RootNamespaceID {
			require.Equal(t, zone, *rt.server.ExecutorConfig().(sql.ExecutorConfig).DefaultZoneConfig)
		}
		if id == keys.RangeEventTableID {
			require.Equal(t, int32(1), zone.GC.TTLSeconds)
			constraints := []zonepb.ConstraintsConjunction{
				{
					Constraints: []zonepb.Constraint{
						{Key: hashedZone, Value: hashedElemName, Type: zonepb.Constraint_REQUIRED},
						{Value: hashedElemName, Type: zonepb.Constraint_REQUIRED},
					},
				},
			}
			require.Equal(t, zone.Constraints, constraints)
		}
		if id == keys.SystemDatabaseID {
			constraints := []zonepb.ConstraintsConjunction{
				{
					NumReplicas: 1,
					Constraints: []zonepb.Constraint{{Value: hashedElemName, Type: zonepb.Constraint_REQUIRED}},
				},
				{
					NumReplicas: 2,
					Constraints: []zonepb.Constraint{
						{Key: hashedZone, Value: hashedElemName, Type: zonepb.Constraint_REQUIRED},
						{Value: hashedElemName, Type: zonepb.Constraint_REQUIRED},
					},
				},
			}
			require.Equal(t, constraints, zone.Constraints)
			prefs := []zonepb.LeasePreference{
				{
					Constraints: []zonepb.Constraint{
						{Key: hashedZone, Value: hashedElemName, Type: zonepb.Constraint_REQUIRED},
						{Value: hashedElemName, Type: zonepb.Constraint_REQUIRED},
					},
				},
				{
					Constraints: []zonepb.Constraint{{Value: hashedElemName, Type: zonepb.Constraint_REQUIRED}},
				},
			}
			require.Equal(t, prefs, zone.LeasePreferences)
		}
	}

	// Verify schema name redaction.
	require.Equal(t, 1, len(last.Schema))
	require.Equal(t, "_", last.Schema[0].Name)
	require.Equal(t, 3, len(last.Schema[0].Columns))
	for _, c := range last.Schema[0].Columns {
		require.Equal(t, "_", c.Name)
	}
}

func TestTelemetry_SuccessfulTelemetryPing(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	defer setTelemetryHttpTimeout(3 * time.Second)()
	rt := startReporterTest(t, base.TestIsSpecificToStorageLayerAndNeedsASystemTenant)
	defer rt.Close()

	ctx := context.Background()
	setupCluster(t, rt.serverDB)

	for _, tc := range []struct {
		name                  string
		respError             error
		respCode              int
		waitSeconds           int
		expectTimestampUpdate bool
	}{
		{
			name:                  "200 response",
			respError:             nil,
			respCode:              200,
			expectTimestampUpdate: true,
		},
		{
			name:                  "400 response",
			respError:             nil,
			respCode:              400,
			expectTimestampUpdate: true,
		},
		{
			name:                  "500 response",
			respError:             nil,
			respCode:              500,
			expectTimestampUpdate: true,
		},
		{
			name:                  "connection error",
			respError:             errors.New("connection refused"),
			expectTimestampUpdate: false,
		},
		{
			name:                  "client timeout",
			respError:             &timeutil.TimeoutError{},
			waitSeconds:           5,
			expectTimestampUpdate: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer rt.diagServer.SetRespError(tc.respError)()
			defer rt.diagServer.SetRespCode(tc.respCode)()
			defer rt.diagServer.SetWaitSeconds(tc.waitSeconds)()
			rt.timesource.Advance(time.Hour)

			dr := rt.server.DiagnosticsReporter().(*diagnostics.Reporter)

			before := rt.timesource.Now().Unix()
			oldTimestamp := dr.LastSuccessfulTelemetryPing.Load()
			require.LessOrEqual(t, dr.LastSuccessfulTelemetryPing.Load(), before)

			rt.timesource.Advance(time.Hour)
			dr.ReportDiagnostics(ctx)

			if tc.expectTimestampUpdate {
				require.Greater(t, dr.LastSuccessfulTelemetryPing.Load(), before)
			} else {
				require.Equal(t, oldTimestamp, dr.LastSuccessfulTelemetryPing.Load())
			}
		})
	}

}

// This test will block on `stopper.Stop` if the diagnostics reporter
// doesn't honor stopper quiescence when making its HTTP request.
func TestTelemetryQuiesce(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	defer setTelemetryHttpTimeout(10 * time.Minute)()
	rt := startReporterTest(t, base.TestIsSpecificToStorageLayerAndNeedsASystemTenant)
	defer rt.Close()

	ctx := context.Background()
	setupCluster(t, rt.serverDB)

	// Ensure that we block for long enough to trigger test timeout.
	defer rt.diagServer.SetWaitSeconds(15 * 60)()
	dr := rt.server.DiagnosticsReporter().(*diagnostics.Reporter)
	stopper := rt.server.Stopper()

	dr.PeriodicallyReportDiagnostics(ctx, stopper)
	stopper.Stop(ctx)
	<-stopper.IsStopped()
}

func TestUsageQuantization(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	defer cloudinfo.Disable()()

	skip.UnderRace(t, "takes >1min under race")
	r := diagutils.NewServer()
	defer r.Close()

	ctx := context.Background()

	url := r.URL()
	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{
		Knobs: base.TestingKnobs{
			Server: &server.TestingKnobs{
				DiagnosticsTestingKnobs: diagnostics.TestingKnobs{
					OverrideReportingURL: &url,
				},
			},
		},
	})
	defer s.Stopper().Stop(ctx)

	// Disable periodic reporting so it doesn't interfere with the test.
	if _, err := db.Exec(`SET CLUSTER SETTING diagnostics.reporting.enabled = false`); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`SET application_name = 'test'`); err != nil {
		t.Fatal(err)
	}

	// Issue some queries against the test app name.
	for i := 0; i < 8; i++ {
		_, err := db.Exec(`SELECT 1`)
		require.NoError(t, err)
	}
	// Between 10 and 100 queries is quantized to 10.
	for i := 0; i < 30; i++ {
		_, err := db.Exec(`SELECT 1,2`)
		require.NoError(t, err)
	}
	// Between 100 and 10000 gets quantized to 100.
	for i := 0; i < 200; i++ {
		_, err := db.Exec(`SELECT 1,2,3`)
		require.NoError(t, err)
	}
	// Above 10000 gets quantized to 10000.
	for i := 0; i < 10010; i++ {
		_, err := db.Exec(`SHOW application_name`)
		require.NoError(t, err)
	}

	ts := s.ApplicationLayer()
	obsConn := sqlutils.MakeSQLRunner(ts.SQLConn(t))

	sqlstatstestutil.WaitForStatementEntriesAtLeast(t, obsConn, 1, sqlstatstestutil.StatementFilter{
		Query:     "SHOW application_name",
		ExecCount: 10010,
	})

	// Flush the SQL stat pool.
	require.NoError(t, ts.SQLServer().(*sql.Server).GetLocalSQLStatsProvider().Reset(ctx))

	// Collect a round of statistics.
	ts.DiagnosticsReporter().(*diagnostics.Reporter).ReportDiagnostics(ctx)

	// The stats "hide" the application name by hashing it. To find the
	// test app name, we need to hash the ref string too prior to the
	// comparison.
	clusterSecret := sql.ClusterSecret.Get(&ts.ClusterSettings().SV)
	hashedAppName := sql.HashForReporting(clusterSecret, "test")
	require.NotEqual(t, sql.FailedHashedValue, hashedAppName, "expected hashedAppName to not be 'unknown'")

	testData := []struct {
		query         string
		expectedCount int64
	}{
		{`SELECT _`, 8},
		{`SELECT _, _`, 10},
		{`SELECT _, _, _`, 100},
		{`SHOW application_name`, 10000},
	}

	last := r.LastRequestData()
	for _, test := range testData {
		found := false
		for _, s := range last.SqlStats {
			if s.Key.App == hashedAppName && s.Key.Query == test.query {
				require.Equal(t, test.expectedCount, s.Stats.Count, "quantization incorrect for query %q", test.query)
				found = true
				break
			}
		}
		if !found {
			t.Errorf("query %q missing from stats", test.query)
		}
	}
}

type reporterTest struct {
	cloudEnable  func()
	settings     *cluster.Settings
	diagServer   *diagutils.Server
	testingKnobs base.TestingKnobs
	serverArgs   base.TestServerArgs
	server       serverutils.TestServerInterface
	serverDB     *gosql.DB
	timesource   *timeutil.ManualTime
}

func (t *reporterTest) Close() {
	t.cloudEnable()
	t.diagServer.Close()
	// stopper will wait for the update/report loop to finish too.
	t.server.Stopper().Stop(context.Background())
}

func startReporterTest(
	t *testing.T, defaultTestTenant base.DefaultTestTenantOptions,
) *reporterTest {
	// Disable cloud info reporting, since it slows down tests.
	rt := &reporterTest{
		cloudEnable: cloudinfo.Disable(),
		settings:    cluster.MakeTestingClusterSettings(),
		diagServer:  diagutils.NewServer(),
		timesource:  timeutil.NewManualTime(timeutil.Now()),
	}

	url := rt.diagServer.URL()
	rt.testingKnobs = base.TestingKnobs{
		SQLLeaseManager: &lease.ManagerTestingKnobs{
			// Disable SELECT called for delete orphaned leases to keep
			// query stats stable.
			DisableDeleteOrphanedLeases: true,
		},
		Server: &server.TestingKnobs{
			DiagnosticsTestingKnobs: diagnostics.TestingKnobs{
				OverrideReportingURL: &url,
				TimeSource:           rt.timesource,
			},
		},
	}

	storeSpec := base.DefaultTestStoreSpec
	storeSpec.Attributes = []string{elemName}
	rt.serverArgs = base.TestServerArgs{
		DefaultTestTenant: defaultTestTenant,
		StoreSpecs: []base.StoreSpec{
			storeSpec,
			base.DefaultTestStoreSpec,
		},
		Settings: rt.settings,
		Locality: roachpb.Locality{
			Tiers: []roachpb.Tier{
				{Key: "region", Value: "east"},
				{Key: "zone", Value: elemName},
				{Key: "state", Value: "ny"},
				{Key: "city", Value: "nyc"},
			},
		},
		Knobs: rt.testingKnobs,
	}
	rt.server, rt.serverDB, _ = serverutils.StartServer(t, rt.serverArgs)

	// Make sure the test's generated activity is the only activity we measure.
	telemetry.GetFeatureCounts(telemetry.Raw, telemetry.ResetCounts)

	// Ensure the org contains "Cockroach Labs" so the telemetry report
	// is marked as "internal".
	_, err := rt.server.SystemLayer().InternalExecutor().(isql.Executor).Exec(
		context.Background(), "set-org", nil,
		`SET CLUSTER SETTING cluster.organization = 'Cockroach Labs - test'`)
	require.NoError(t, err)

	return rt
}

func setupCluster(t *testing.T, db *gosql.DB) {
	_, err := db.Exec(`SET CLUSTER SETTING server.time_until_store_dead = '90s'`)
	require.NoError(t, err)

	// Enable diagnostics reporting to test PeriodicallyReportDiagnostics.
	_, err = db.Exec(`SET CLUSTER SETTING diagnostics.reporting.enabled = true`)
	require.NoError(t, err)

	_, err = db.Exec(`SET CLUSTER SETTING diagnostics.reporting.send_crash_reports.enabled = false`)
	require.NoError(t, err)

	_, err = db.Exec(fmt.Sprintf(`CREATE DATABASE %s`, elemName))
	require.NoError(t, err)

	q := `SET CLUSTER SETTING cluster.label = 'Some String'`
	_, err = db.Exec(q)
	require.NoError(t, err)
}

func verifyEnvironment(
	t *testing.T, secret string, locality roachpb.Locality, env *diagnosticspb.Environment,
) {
	require.NotEqual(t, 0, env.Hardware.Mem.Total)
	require.NotEqual(t, 0, env.Hardware.Mem.Available)
	require.Equal(t, int32(system.NumCPU()), env.Hardware.Cpu.Numcpu)
	require.NotEqual(t, 0, env.Hardware.Cpu.Sockets)
	require.NotEqual(t, 0.0, env.Hardware.Cpu.Mhz)
	require.NotEqual(t, 0.0, env.Os.Platform)
	require.NotEmpty(t, env.Build.Tag)
	require.NotEmpty(t, env.Build.Distribution)
	require.NotEmpty(t, env.LicenseType)

	require.Equal(t, len(locality.Tiers), len(env.Locality.Tiers))
	for i := range locality.Tiers {
		require.Equal(t, sql.HashForReporting(secret, locality.Tiers[i].Key), env.Locality.Tiers[i].Key)
		require.Equal(t, sql.HashForReporting(secret, locality.Tiers[i].Value), env.Locality.Tiers[i].Value)
	}
}
