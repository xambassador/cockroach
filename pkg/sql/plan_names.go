// Copyright 2016 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package sql

import (
	"reflect"

	"github.com/cockroachdb/errors"
)

// nodeName returns the name of the given planNode as string.  The
// node's current state is taken into account, e.g. sortNode has
// either name "sort" or "nosort" depending on whether sorting is
// needed.
func nodeName(plan planNode) string {
	// Some nodes have custom names depending on attributes.
	switch n := plan.(type) {
	case *scanNode:
		if n.reverse {
			return "revscan"
		}

	case *joinNode:
		if len(n.mergeJoinOrdering) > 0 {
			return "merge join"
		}
		if len(n.pred.leftEqualityIndices) == 0 {
			return "cross join"
		}
		return "hash join"
	}

	name, ok := planNodeNames[reflect.TypeOf(plan)]
	if !ok {
		panic(errors.AssertionFailedf("name missing for type %T", plan))
	}

	return name
}

// planNodeNames is the mapping from node type to strings.  The
// strings are constant and not precomputed so that the type names can
// be changed without changing the output of "EXPLAIN".
var planNodeNames = map[reflect.Type]string{
	reflect.TypeOf(&alterDatabaseOwnerNode{}):                  "alter database owner",
	reflect.TypeOf(&alterDatabaseAddRegionNode{}):              "alter database add region",
	reflect.TypeOf(&alterDatabasePrimaryRegionNode{}):          "alter database primary region",
	reflect.TypeOf(&alterDatabasePlacementNode{}):              "alter database placement",
	reflect.TypeOf(&alterDatabaseSurvivalGoalNode{}):           "alter database survive",
	reflect.TypeOf(&alterDatabaseDropRegionNode{}):             "alter database drop region",
	reflect.TypeOf(&alterDatabaseAddSuperRegion{}):             "alter database add super region",
	reflect.TypeOf(&alterDatabaseDropSuperRegion{}):            "alter database alter super region",
	reflect.TypeOf(&alterDatabaseAlterSuperRegion{}):           "alter database drop super region",
	reflect.TypeOf(&alterDatabaseSecondaryRegion{}):            "alter database secondary region",
	reflect.TypeOf(&alterDatabaseDropSecondaryRegion{}):        "alter database secondary region",
	reflect.TypeOf(&alterDatabaseSetZoneConfigExtensionNode{}): "alter database configure zone extension",
	reflect.TypeOf(&alterDefaultPrivilegesNode{}):              "alter default privileges",
	reflect.TypeOf(&alterExternalConnectionNode{}):             "alter external connection",
	reflect.TypeOf(&alterFunctionOptionsNode{}):                "alter function",
	reflect.TypeOf(&alterFunctionRenameNode{}):                 "alter function rename",
	reflect.TypeOf(&alterFunctionSetOwnerNode{}):               "alter function owner",
	reflect.TypeOf(&alterFunctionSetSchemaNode{}):              "alter function set schema",
	reflect.TypeOf(&alterFunctionDepExtensionNode{}):           "alter function depends on extension",
	reflect.TypeOf(&alterIndexNode{}):                          "alter index",
	reflect.TypeOf(&alterIndexVisibleNode{}):                   "alter index visibility",
	reflect.TypeOf(&alterJobOwnerNode{}):                       "alter job owner",
	reflect.TypeOf(&alterSequenceNode{}):                       "alter sequence",
	reflect.TypeOf(&alterSchemaNode{}):                         "alter schema",
	reflect.TypeOf(&alterTableNode{}):                          "alter table",
	reflect.TypeOf(&alterTableOwnerNode{}):                     "alter table owner",
	reflect.TypeOf(&alterTableSetLocalityNode{}):               "alter table set locality",
	reflect.TypeOf(&alterTableSetSchemaNode{}):                 "alter table set schema",
	reflect.TypeOf(&alterTenantCapabilityNode{}):               "alter tenant capability",
	reflect.TypeOf(&alterTenantSetClusterSettingNode{}):        "alter tenant set cluster setting",
	reflect.TypeOf(&alterTenantServiceNode{}):                  "alter tenant service",
	reflect.TypeOf(&alterTypeNode{}):                           "alter type",
	reflect.TypeOf(&alterRoleNode{}):                           "alter role",
	reflect.TypeOf(&alterRoleSetNode{}):                        "alter role set var",
	reflect.TypeOf(&applyJoinNode{}):                           "apply join",
	reflect.TypeOf(&bufferNode{}):                              "buffer",
	reflect.TypeOf(&callNode{}):                                "call",
	reflect.TypeOf(&cancelQueriesNode{}):                       "cancel queries",
	reflect.TypeOf(&cancelSessionsNode{}):                      "cancel sessions",
	reflect.TypeOf(&cdcValuesNode{}):                           "wrapped streaming node",
	reflect.TypeOf(&changeDescriptorBackedPrivilegesNode{}):    "change privileges",
	reflect.TypeOf(&changeNonDescriptorBackedPrivilegesNode{}): "change system privileges",
	reflect.TypeOf(&commentOnColumnNode{}):                     "comment on column",
	reflect.TypeOf(&commentOnConstraintNode{}):                 "comment on constraint",
	reflect.TypeOf(&commentOnDatabaseNode{}):                   "comment on database",
	reflect.TypeOf(&commentOnIndexNode{}):                      "comment on index",
	reflect.TypeOf(&commentOnTableNode{}):                      "comment on table",
	reflect.TypeOf(&commentOnSchemaNode{}):                     "comment on schema",
	reflect.TypeOf(&completionsNode{}):                         "show completions",
	reflect.TypeOf(&controlJobsNode{}):                         "control jobs",
	reflect.TypeOf(&controlSchedulesNode{}):                    "control schedules",
	reflect.TypeOf(&createDatabaseNode{}):                      "create database",
	reflect.TypeOf(&createExtensionNode{}):                     "create extension",
	reflect.TypeOf(&createExternalConnectionNode{}):            "create external connection",
	reflect.TypeOf(&createFunctionNode{}):                      "create function",
	reflect.TypeOf(&createIndexNode{}):                         "create index",
	reflect.TypeOf(&createSequenceNode{}):                      "create sequence",
	reflect.TypeOf(&createSchemaNode{}):                        "create schema",
	reflect.TypeOf(&createStatsNode{}):                         "create statistics",
	reflect.TypeOf(&createTableNode{}):                         "create table",
	reflect.TypeOf(&createTenantNode{}):                        "create tenant",
	reflect.TypeOf(&createTypeNode{}):                          "create type",
	reflect.TypeOf(&CreateRoleNode{}):                          "create user/role",
	reflect.TypeOf(&createViewNode{}):                          "create view",
	reflect.TypeOf(&checkExternalConnectionNode{}):             "check external connection",
	reflect.TypeOf(&delayedNode{}):                             "virtual table",
	reflect.TypeOf(&deleteNode{}):                              "delete",
	reflect.TypeOf(&deleteRangeNode{}):                         "delete range",
	reflect.TypeOf(&deleteSwapNode{}):                          "delete swap",
	reflect.TypeOf(&discardNode{}):                             "discard",
	reflect.TypeOf(&distinctNode{}):                            "distinct",
	reflect.TypeOf(&dropDatabaseNode{}):                        "drop database",
	reflect.TypeOf(&dropExternalConnectionNode{}):              "drop external connection",
	reflect.TypeOf(&dropFunctionNode{}):                        "drop function",
	reflect.TypeOf(&dropIndexNode{}):                           "drop index",
	reflect.TypeOf(&dropSequenceNode{}):                        "drop sequence",
	reflect.TypeOf(&dropSchemaNode{}):                          "drop schema",
	reflect.TypeOf(&dropTableNode{}):                           "drop table",
	reflect.TypeOf(&dropTenantNode{}):                          "drop tenant",
	reflect.TypeOf(&dropTypeNode{}):                            "drop type",
	reflect.TypeOf(&DropRoleNode{}):                            "drop user/role",
	reflect.TypeOf(&dropViewNode{}):                            "drop view",
	reflect.TypeOf(&errorIfRowsNode{}):                         "error if rows",
	reflect.TypeOf(&explainPlanNode{}):                         "explain plan",
	reflect.TypeOf(&explainVecNode{}):                          "explain vectorized",
	reflect.TypeOf(&explainDDLNode{}):                          "explain ddl",
	reflect.TypeOf(&exportNode{}):                              "export",
	reflect.TypeOf(&fetchNode{}):                               "fetch",
	reflect.TypeOf(&filterNode{}):                              "filter",
	reflect.TypeOf(&endPreparedTxnNode{}):                      "commit/rollback prepared",
	reflect.TypeOf(&GrantRoleNode{}):                           "grant role",
	reflect.TypeOf(&groupNode{}):                               "group",
	reflect.TypeOf(&hookFnNode{}):                              "plugin",
	reflect.TypeOf(&indexJoinNode{}):                           "index join",
	reflect.TypeOf(&insertNode{}):                              "insert",
	reflect.TypeOf(&insertFastPathNode{}):                      "insert fast path",
	reflect.TypeOf(&invertedFilterNode{}):                      "inverted filter",
	reflect.TypeOf(&invertedJoinNode{}):                        "inverted join",
	reflect.TypeOf(&joinNode{}):                                "join",
	reflect.TypeOf(&limitNode{}):                               "limit",
	reflect.TypeOf(&lookupJoinNode{}):                          "lookup join",
	reflect.TypeOf(&max1RowNode{}):                             "max1row",
	reflect.TypeOf(&ordinalityNode{}):                          "ordinality",
	reflect.TypeOf(&projectSetNode{}):                          "project set",
	reflect.TypeOf(&reassignOwnedByNode{}):                     "reassign owned by",
	reflect.TypeOf(&recursiveCTENode{}):                        "recursive cte",
	reflect.TypeOf(&refreshMaterializedViewNode{}):             "refresh materialized view",
	reflect.TypeOf(&relocateNode{}):                            "relocate",
	reflect.TypeOf(&relocateRange{}):                           "relocate range",
	reflect.TypeOf(&renameColumnNode{}):                        "rename column",
	reflect.TypeOf(&renameDatabaseNode{}):                      "rename database",
	reflect.TypeOf(&renameIndexNode{}):                         "rename index",
	reflect.TypeOf(&renameTableNode{}):                         "rename table",
	reflect.TypeOf(&renameTenantNode{}):                        "rename tenant",
	reflect.TypeOf(&renderNode{}):                              "render",
	reflect.TypeOf(&resetAllNode{}):                            "reset all",
	reflect.TypeOf(&RevokeRoleNode{}):                          "revoke role",
	reflect.TypeOf(&rowCountNode{}):                            "count",
	reflect.TypeOf(&rowSourceToPlanNode{}):                     "row source to plan node",
	reflect.TypeOf(&saveTableNode{}):                           "save table",
	reflect.TypeOf(&scanBufferNode{}):                          "scan buffer",
	reflect.TypeOf(&scanNode{}):                                "scan",
	reflect.TypeOf(&scatterNode{}):                             "scatter",
	reflect.TypeOf(&scrubNode{}):                               "scrub",
	reflect.TypeOf(&sequenceSelectNode{}):                      "sequence select",
	reflect.TypeOf(&serializeNode{}):                           "run",
	reflect.TypeOf(&setClusterSettingNode{}):                   "set cluster setting",
	reflect.TypeOf(&setSessionAuthorizationDefaultNode{}):      "set session authorization",
	reflect.TypeOf(&setVarNode{}):                              "set",
	reflect.TypeOf(&setZoneConfigNode{}):                       "configure zone",
	reflect.TypeOf(&showFingerprintsNode{}):                    "show fingerprints",
	reflect.TypeOf(&showTenantNode{}):                          "show tenant",
	reflect.TypeOf(&showTraceNode{}):                           "show trace for",
	reflect.TypeOf(&showTraceReplicaNode{}):                    "replica trace",
	reflect.TypeOf(&showVarNode{}):                             "show",
	reflect.TypeOf(&sortNode{}):                                "sort",
	reflect.TypeOf(&splitNode{}):                               "split",
	reflect.TypeOf(&topKNode{}):                                "top-k",
	reflect.TypeOf(&unsplitNode{}):                             "unsplit",
	reflect.TypeOf(&unsplitAllNode{}):                          "unsplit all",
	reflect.TypeOf(&spoolNode{}):                               "spool",
	reflect.TypeOf(&truncateNode{}):                            "truncate",
	reflect.TypeOf(&unaryNode{}):                               "emptyrow",
	reflect.TypeOf(&unionNode{}):                               "union",
	reflect.TypeOf(&updateNode{}):                              "update",
	reflect.TypeOf(&updateSwapNode{}):                          "update swap",
	reflect.TypeOf(&upsertNode{}):                              "upsert",
	reflect.TypeOf(&valuesNode{}):                              "values",
	reflect.TypeOf(&vectorMutationSearchNode{}):                "vector mutation search",
	reflect.TypeOf(&vectorSearchNode{}):                        "vector search",
	reflect.TypeOf(&virtualTableNode{}):                        "virtual table values",
	reflect.TypeOf(&vTableLookupJoinNode{}):                    "virtual table lookup join",
	reflect.TypeOf(&windowNode{}):                              "window",
	reflect.TypeOf(&zeroNode{}):                                "norows",
	reflect.TypeOf(&zigzagJoinNode{}):                          "zigzag join",
	reflect.TypeOf(&schemaChangePlanNode{}):                    "schema change",
	reflect.TypeOf(&identifySystemNode{}):                      "identify system",
}
