// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package catformat

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/geo/geoindex"
	"github.com/cockroachdb/cockroach/pkg/geo/geopb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemaexpr"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/idxtype"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/vecindex/vecpb"
	"github.com/cockroachdb/errors"
)

// IndexDisplayMode influences how an index should be formatted for pretty print
// in IndexForDisplay function.
type IndexDisplayMode int

const (
	// IndexDisplayShowCreate indicates index definition to be printed as a CREATE
	// INDEX statement.
	IndexDisplayShowCreate IndexDisplayMode = iota
	// IndexDisplayDefOnly indicates index definition to be printed as INDEX
	// definition format within a CREATE TABLE statement.
	IndexDisplayDefOnly
)

// IndexForDisplay formats an index descriptor as a SQL string. It converts user
// defined types in partial index predicate expressions to a human-readable
// form.
//
// If tableName is anonymous then no table name is included in the formatted
// string. For example:
//
//	INDEX i (a) WHERE b > 0
//
// If tableName is not anonymous, then "ON" and the name is included:
//
//	INDEX i ON t (a) WHERE b > 0
func IndexForDisplay(
	ctx context.Context,
	table catalog.TableDescriptor,
	tableName *tree.TableName,
	index catalog.Index,
	partition string,
	formatFlags tree.FmtFlags,
	evalCtx *eval.Context,
	semaCtx *tree.SemaContext,
	sessionData *sessiondata.SessionData,
	displayMode IndexDisplayMode,
) (string, error) {
	return indexForDisplay(
		ctx,
		table,
		tableName,
		index.IndexDesc(),
		index.Primary(),
		partition,
		formatFlags,
		evalCtx,
		semaCtx,
		sessionData,
		displayMode,
	)
}

func indexForDisplay(
	ctx context.Context,
	table catalog.TableDescriptor,
	tableName *tree.TableName,
	index *descpb.IndexDescriptor,
	isPrimary bool,
	partition string,
	formatFlags tree.FmtFlags,
	evalCtx *eval.Context,
	semaCtx *tree.SemaContext,
	sessionData *sessiondata.SessionData,
	displayMode IndexDisplayMode,
) (string, error) {
	// Please also update CreateIndex's "Format" method in
	// pkg/sql/sem/tree/create.go if there's any update to index definition
	// components.
	if displayMode == IndexDisplayShowCreate && *tableName == descpb.AnonymousTable {
		return "", errors.New("tableName must be set for IndexDisplayShowCreate mode")
	}

	f := tree.NewFmtCtx(formatFlags)
	if displayMode == IndexDisplayShowCreate {
		f.WriteString("CREATE ")
	}
	displayPrimaryKeyClauses := isPrimary && displayMode == IndexDisplayDefOnly
	if index.Unique && !displayPrimaryKeyClauses {
		f.WriteString("UNIQUE ")
	}
	if !f.HasFlags(tree.FmtPGCatalog) {
		switch index.Type {
		case idxtype.INVERTED:
			f.WriteString("INVERTED ")
		case idxtype.VECTOR:
			f.WriteString("VECTOR ")
		}
	}
	if displayPrimaryKeyClauses {
		f.WriteString("PRIMARY KEY")
	} else {
		f.WriteString("INDEX ")
		f.FormatNameP(&index.Name)
	}
	if *tableName != descpb.AnonymousTable {
		f.WriteString(" ON ")
		f.FormatNode(tableName)
	}

	if f.HasFlags(tree.FmtPGCatalog) {
		f.WriteString(" USING")
		switch index.Type {
		case idxtype.INVERTED:
			f.WriteString(" gin")
		case idxtype.VECTOR:
			f.WriteString(" cspann")
		default:
			f.WriteString(" btree")
		}
	}

	f.WriteString(" (")
	if err := FormatIndexElements(ctx, table, index, f, evalCtx, semaCtx, sessionData); err != nil {
		return "", err
	}
	f.WriteByte(')')

	if index.IsSharded() {
		if f.HasFlags(tree.FmtPGCatalog) {
			fmt.Fprintf(f, " USING HASH WITH (bucket_count=%v)",
				index.Sharded.ShardBuckets)
		} else {
			f.WriteString(" USING HASH")
		}
	}

	if !isPrimary && len(index.StoreColumnNames) > 0 {
		f.WriteString(" STORING (")
		for i := range index.StoreColumnNames {
			if i > 0 {
				f.WriteString(", ")
			}
			f.FormatNameP(&index.StoreColumnNames[i])
		}
		f.WriteByte(')')
	}

	f.WriteString(partition)

	if !f.HasFlags(tree.FmtPGCatalog) {
		if err := formatStorageConfigs(table, index, f); err != nil {
			return "", err
		}
	}

	if index.IsPartial() {
		predFmtFlag := tree.FmtParsable
		if f.HasFlags(tree.FmtPGCatalog) {
			predFmtFlag = tree.FmtPGCatalog
		} else {
			if f.HasFlags(tree.FmtMarkRedactionNode) {
				predFmtFlag |= tree.FmtMarkRedactionNode
			}
			if f.HasFlags(tree.FmtOmitNameRedaction) {
				predFmtFlag |= tree.FmtOmitNameRedaction
			}
		}
		pred, err := schemaexpr.FormatExprForDisplay(ctx, table, index.Predicate, evalCtx, semaCtx, sessionData, predFmtFlag)
		if err != nil {
			return "", err
		}

		f.WriteString(" WHERE ")
		if f.HasFlags(tree.FmtPGCatalog) {
			f.WriteString("(")
			f.WriteString(pred)
			f.WriteString(")")
		} else {
			f.WriteString(pred)
		}
	}

	if idxInvisibility := index.Invisibility; idxInvisibility != 0.0 {
		if idxInvisibility == 1.0 {
			f.WriteString(" NOT VISIBLE")
		} else {
			f.WriteString(" VISIBILITY ")
			f.WriteString(fmt.Sprintf("%.2f", 1-index.Invisibility))
		}
	}

	return f.CloseAndGetString(), nil
}

// FormatIndexElements formats the key columns an index. If the column is an
// inaccessible computed column, the computed column expression is formatted.
// Otherwise, the column name is formatted. Each column is separated by commas
// and includes the direction of the index if the index is not an inverted
// index.
func FormatIndexElements(
	ctx context.Context,
	table catalog.TableDescriptor,
	index *descpb.IndexDescriptor,
	f *tree.FmtCtx,
	evalCtx *eval.Context,
	semaCtx *tree.SemaContext,
	sessionData *sessiondata.SessionData,
) error {
	elemFmtFlag := tree.FmtParsable
	if f.HasFlags(tree.FmtPGCatalog) {
		elemFmtFlag = tree.FmtPGCatalog
	} else {
		if f.HasFlags(tree.FmtMarkRedactionNode) {
			elemFmtFlag |= tree.FmtMarkRedactionNode
		}
		if f.HasFlags(tree.FmtOmitNameRedaction) {
			elemFmtFlag |= tree.FmtOmitNameRedaction
		}
	}

	startIdx := index.ExplicitColumnStartIdx()
	for i, n := startIdx, len(index.KeyColumnIDs); i < n; i++ {
		col, err := catalog.MustFindColumnByID(table, index.KeyColumnIDs[i])
		if err != nil {
			return err
		}
		if i > startIdx {
			f.WriteString(", ")
		}
		if col.IsExpressionIndexColumn() {
			expr, err := schemaexpr.FormatExprForExpressionIndexDisplay(
				ctx, table, col.GetComputeExpr(), evalCtx, semaCtx, sessionData, elemFmtFlag,
			)
			if err != nil {
				return err
			}
			f.WriteString(expr)
		} else {
			f.FormatNameP(&index.KeyColumnNames[i])
		}
		switch index.Type {
		case idxtype.INVERTED:
			if col.GetID() == index.InvertedColumnID() && len(index.InvertedColumnKinds) > 0 {
				switch index.InvertedColumnKinds[0] {
				case catpb.InvertedIndexColumnKind_TRIGRAM:
					f.WriteString(" gin_trgm_ops")
				}
			}
		case idxtype.VECTOR:
			if col.GetID() == index.VectorColumnID() {
				switch index.VecConfig.DistanceMetric {
				case vecpb.L2SquaredDistance:
					f.WriteString(" vector_l2_ops")
				case vecpb.CosineDistance:
					f.WriteString(" vector_cosine_ops")
				case vecpb.InnerProductDistance:
					f.WriteString(" vector_ip_ops")
				}
			}
		}
		// Vector indexes do not support ASC/DESC modifiers.
		if !index.Type.HasScannablePrefix() {
			continue
		}
		// The last column of an inverted index cannot have a DESC direction
		// because it does not have a linear ordering. Since the default
		// direction is ASC, we omit the direction entirely for inverted index
		// columns.
		if i < n-1 || index.Type.HasLinearOrdering() {
			f.WriteByte(' ')
			f.WriteString(index.KeyColumnDirections[i].String())
		}
	}
	return nil
}

// formatStorageConfigs writes the index's storage configurations to the given
// format context.
func formatStorageConfigs(
	table catalog.TableDescriptor, index *descpb.IndexDescriptor, f *tree.FmtCtx,
) error {
	numCustomSettings := 0
	writeCustomSetting := func(key, val string) {
		if numCustomSettings > 0 {
			f.WriteString(", ")
		} else {
			f.WriteString(" WITH (")
		}
		numCustomSettings++
		f.WriteString(key)
		f.WriteString("=")
		f.WriteString(val)
	}

	if index.GeoConfig.S2Geometry != nil || index.GeoConfig.S2Geography != nil {
		var s2Config *geopb.S2Config

		if index.GeoConfig.S2Geometry != nil {
			s2Config = index.GeoConfig.S2Geometry.S2Config
		}
		if index.GeoConfig.S2Geography != nil {
			s2Config = index.GeoConfig.S2Geography.S2Config
		}

		defaultS2Config := geoindex.DefaultS2Config()
		if *s2Config != *defaultS2Config {
			for _, check := range []struct {
				key        string
				val        int32
				defaultVal int32
			}{
				{`s2_max_level`, s2Config.MaxLevel, defaultS2Config.MaxLevel},
				{`s2_level_mod`, s2Config.LevelMod, defaultS2Config.LevelMod},
				{`s2_max_cells`, s2Config.MaxCells, defaultS2Config.MaxCells},
			} {
				if check.val != check.defaultVal {
					writeCustomSetting(check.key, strconv.Itoa(int(check.val)))
				}
			}
		}

		if index.GeoConfig.S2Geometry != nil {
			col, err := catalog.MustFindColumnByID(table, index.InvertedColumnID())
			if err != nil {
				return errors.Wrapf(err, "expected column %q to exist in table", index.InvertedColumnName())
			}
			defaultConfig, err := geoindex.GeometryIndexConfigForSRID(col.GetType().GeoSRIDOrZero())
			if err != nil {
				return errors.Wrapf(err, "expected SRID definition for %d", col.GetType().GeoSRIDOrZero())
			}
			cfg := index.GeoConfig.S2Geometry

			for _, check := range []struct {
				key        string
				val        float64
				defaultVal float64
			}{
				{`geometry_min_x`, cfg.MinX, defaultConfig.S2Geometry.MinX},
				{`geometry_max_x`, cfg.MaxX, defaultConfig.S2Geometry.MaxX},
				{`geometry_min_y`, cfg.MinY, defaultConfig.S2Geometry.MinY},
				{`geometry_max_y`, cfg.MaxY, defaultConfig.S2Geometry.MaxY},
			} {
				if check.val != check.defaultVal {
					writeCustomSetting(check.key, strconv.FormatFloat(check.val, 'f', -1, 64))
				}
			}
		}
	}

	if index.Type == idxtype.VECTOR {
		if index.VecConfig.BuildBeamSize != 0 {
			writeCustomSetting(`build_beam_size`, strconv.Itoa(int(index.VecConfig.BuildBeamSize)))
		}
		if index.VecConfig.MinPartitionSize != 0 {
			writeCustomSetting(`min_partition_size`, strconv.Itoa(int(index.VecConfig.MinPartitionSize)))
		}
		if index.VecConfig.MaxPartitionSize != 0 {
			writeCustomSetting(`max_partition_size`, strconv.Itoa(int(index.VecConfig.MaxPartitionSize)))
		}
	}

	if index.IsSharded() {
		writeCustomSetting(`bucket_count`, strconv.FormatInt(int64(index.Sharded.ShardBuckets), 10))
	}

	if numCustomSettings > 0 {
		f.WriteString(")")
	}

	return nil
}
