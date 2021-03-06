package postgis

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/provider"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/pgtype"
	ent "github.com/SharperShape/entity-id"
)

// genSQL will fill in the SQL field of a layer given a pool, and list of fields.
func genSQL(l *Layer, pool *pgx.ConnPool, tblname string, flds []string, buffer bool) (sql string, err error) {

	// we need to hit the database to see what the fields are.
	if len(flds) == 0 {
		sql := fmt.Sprintf(fldsSQL, tblname)

		//	if a subquery is set in the 'sql' config the subquery is set to the layer's
		//	'tablename' param. because of this case normal SQL token replacement needs to be
		//	applied to tablename SQL generation

		tile := provider.NewTile(0, 0, 0, 64, tegola.WebMercator)
		sql, err = replaceTokens(nil, sql, l, tile, buffer)
		if err != nil {
			return "", err
		}

		rows, err := pool.Query(sql)
		if err != nil {
			return "", err
		}
		defer rows.Close()

		fdescs := rows.FieldDescriptions()
		if len(fdescs) == 0 {
			return "", fmt.Errorf("No fields were returned for table %v", tblname)
		}

		// to avoid field names possibly colliding with Postgres keywords,
		// we wrap the field names in quotes
		for i := range fdescs {
			flds = append(flds, fdescs[i].Name)
		}
	}

	fgeom := -1

	for i, f := range flds {
		if f == l.geomField {
			fgeom = i
		}
		flds[i] = fmt.Sprintf(`"%v"`, flds[i])
	}

	// to avoid field names possibly colliding with Postgres keywords,
	// we wrap the field names in quotes
	if fgeom == -1 {
		flds = append(flds, fmt.Sprintf(`ST_AsBinary("%v") AS "%[1]v"`, l.geomField))
	} else {
		flds[fgeom] = fmt.Sprintf(`ST_AsBinary("%v") AS "%[1]v"`, l.geomField)
	}

	// add required id field
	if l.idField != "" {
		flds = append(flds, fmt.Sprintf(`"%v"`, l.idField))
	}

	selectClause := strings.Join(flds, ", ")

	return fmt.Sprintf(stdSQL, selectClause, tblname, l.geomField), nil
}

const (
	bboxToken             = "!BBOX!"
	zoomToken             = "!ZOOM!"
	xToken                = "!X!"
	yToken                = "!Y!"
	zToken                = "!Z!"
	scaleDenominatorToken = "!SCALE_DENOMINATOR!"
	pixelWidthToken       = "!PIXEL_WIDTH!"
	pixelHeightToken      = "!PIXEL_HEIGHT!"
	idFieldToken          = "!ID_FIELD!"
	geomFieldToken        = "!GEOM_FIELD!"
	geomTypeToken         = "!GEOM_TYPE!"
	companyIDToken        = "!COMPANY_ID!"
	projectIDsToken 	  = "!PROJECT_IDS!"
	projectIDToken        = "!PROJECT_ID!"
)

// replaceTokens replaces tokens in the provided SQL string
//
// !BBOX! - the bounding box of the tile
// !ZOOM! - the tile Z value
// !X! - the tile X value
// !Y! - the tile Y value
// !Z! - the tile Z value
// !SCALE_DENOMINATOR! - scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size)
// !PIXEL_WIDTH! - the pixel width in meters, assuming 256x256 tiles
// !PIXEL_HEIGHT! - the pixel height in meters, assuming 256x256 tiles
// !GEOM_FIELD! - the geom field name
// !GEOM_TYPE! - the geom field type if defined otherwise ""
// !COMPANY_ID! - the company id to restrict the scope to
// !PROJECT_IDS! - the project id to restrict the scope to (project_ids as an array in table)
// !PROJECT_ID! - the project id to restrict the scope to (single project entity)
func replaceTokens(ctx context.Context, sql string, lyr *Layer, tile provider.Tile, withBuffer bool) (string, error) {
	var (
		extent  *geom.Extent
		geoType string
	)

	if lyr == nil {
		return "", ErrNilLayer
	}
	srid := lyr.SRID()

	if withBuffer {
		extent, _ = tile.BufferedExtent()
	} else {
		extent, _ = tile.Extent()
	}

	// TODO: leverage helper functions for minx / miny to make this easier to follow
	// TODO: it's currently assumed the tile will always be in WebMercator. Need to support different projections
	minGeo, err := basic.FromWebMercator(srid, geom.Point{extent.MinX(), extent.MinY()})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}

	maxGeo, err := basic.FromWebMercator(srid, geom.Point{extent.MaxX(), extent.MaxY()})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}

	minPt, maxPt := minGeo.(geom.Point), maxGeo.(geom.Point)

	bbox := fmt.Sprintf("ST_MakeEnvelope(%g,%g,%g,%g,%d)", minPt.X(), minPt.Y(), maxPt.X(), maxPt.Y(), srid)

	extent, _ = tile.Extent()
	// TODO: Always convert to meter if we support different projections
	pixelWidth := (extent.MaxX() - extent.MinX()) / 256
	pixelHeight := (extent.MaxY() - extent.MinY()) / 256
	scaleDenominator := pixelWidth / 0.00028 /* px size in m */

	if lyr.GeomType() != nil {
		geoType = fmt.Sprintf("%v", lyr.GeomType())
	}

	// replace query string tokens
	z, x, y := tile.ZXY()
	tokenReplacer := strings.NewReplacer(
		bboxToken, bbox,
		zoomToken, strconv.FormatUint(uint64(z), 10),
		zToken, strconv.FormatUint(uint64(z), 10),
		xToken, strconv.FormatUint(uint64(x), 10),
		yToken, strconv.FormatUint(uint64(y), 10),
		idFieldToken, lyr.IDFieldName(),
		geomFieldToken, lyr.GeomFieldName(),
		geomTypeToken, geoType,
		scaleDenominatorToken, strconv.FormatFloat(scaleDenominator, 'f', -1, 64),
		pixelWidthToken, strconv.FormatFloat(pixelWidth, 'f', -1, 64),
		pixelHeightToken, strconv.FormatFloat(pixelHeight, 'f', -1, 64),
	)

	uppercaseTokenSQL := uppercaseTokens(sql)

	replaced := tokenReplacer.Replace(uppercaseTokenSQL)




	if ctx != nil {
		companyID, err := validateAndFormatID(ctx.Value("companyId").(string))
		if err != nil {
			return "", err
		}
		replaced = strings.ReplaceAll(replaced, companyIDToken, fmt.Sprintf(`company_id=%s`, companyID))
	} else {
		replaced = strings.ReplaceAll(replaced, fmt.Sprintf("%s AND ", companyIDToken), "")
	}

	var projectIDs []string
	if ctx != nil {
		p := ctx.Value("projectIDs")
		if p != nil {
			for _, rawPID := range p.([]string) {
				pID, err := validateAndFormatID(rawPID)
				if err != nil {
					return "", err
				}
				projectIDs = append(projectIDs, "BYTEA(" + pID + ")")
			}
		}
	}

	if projectIDs != nil {
		replaced = strings.ReplaceAll(replaced, projectIDToken, fmt.Sprintf(`project_id in (%s)`, strings.Join(projectIDs, ",")))
		replaced = strings.ReplaceAll(replaced, projectIDsToken, fmt.Sprintf(`ARRAY[%s] && project_ids`, strings.Join(projectIDs, ",")))
	} else {
		replaced = strings.ReplaceAll(replaced, fmt.Sprintf("%s AND ", projectIDsToken), "")
	}


	return replaced, nil
}

func validateAndFormatID(input string) (string, error) {
	_, err := ent.ParseHex(input)
	return fmt.Sprintf(`'\x%s'`, input), err
}

var tokenRe = regexp.MustCompile("![a-zA-Z0-9_-]+!")

//	uppercaseTokens converts all !tokens! to uppercase !TOKENS!. Tokens can
//	contain alphanumerics, dash and underline chars.
func uppercaseTokens(str string) string {
	return tokenRe.ReplaceAllStringFunc(str, strings.ToUpper)
}

func transformVal(valType pgtype.OID, val interface{}) (interface{}, error) {
	switch valType {
	default:
		switch vt := val.(type) {
		default:
			log.Printf("%v type is not supported. (Expected it to be a stringer type)", valType)
			return nil, fmt.Errorf("%v type is not supported. (Expected it to be a stringer type)", valType)
		case fmt.Stringer:
			return vt.String(), nil
		case string:
			return vt, nil
		}
	case pgtype.BoolOID, pgtype.ByteaOID, pgtype.TextOID, pgtype.OIDOID, pgtype.VarcharOID, pgtype.JSONBOID:
		return val, nil
	case pgtype.Int8OID, pgtype.Int2OID, pgtype.Int4OID, pgtype.Float4OID, pgtype.Float8OID:
		switch vt := val.(type) {
		case int8:
			return int64(vt), nil
		case int16:
			return int64(vt), nil
		case int32:
			return int64(vt), nil
		case int64, uint64:
			return vt, nil
		case uint8:
			return int64(vt), nil
		case uint16:
			return int64(vt), nil
		case uint32:
			return int64(vt), nil
		case float32:
			return float64(vt), nil
		case float64:
			return vt, nil
		default: // should never happen.
			return nil, fmt.Errorf("%v type is not supported. (should never happen)", valType)
		}
	case pgtype.DateOID, pgtype.TimestampOID, pgtype.TimestamptzOID:
		return fmt.Sprintf("%v", val), nil
	}
}

// decipherFields is responsible for processing the SQL result set, decoding geometries, ids and feature tags.
func decipherFields(ctx context.Context, geomFieldname, idFieldname string, descriptions []pgx.FieldDescription, values []interface{}) (gid uint64, geom []byte, tags map[string]interface{}, err error) {
	var ok bool

	tags = make(map[string]interface{})

	var idParsed bool
	for i := range values {
		// do a quick check
		if err := ctx.Err(); err != nil {
			return 0, nil, nil, err
		}

		// skip nil values.
		if values[i] == nil {
			continue
		}

		desc := descriptions[i]

		switch desc.Name {
		case geomFieldname:
			if geom, ok = values[i].([]byte); !ok {
				return 0, nil, nil, fmt.Errorf("unable to convert geometry field (%v) into bytes.", geomFieldname)
			}
		case idFieldname:
			// the id has to be parsed once but it can also be a tag
			if !idParsed {
				gid, err = gId(values[i])
				if err != nil {
					return 0, nil, nil, err
				}
				idParsed = true
				break
			}

			// adds id as a tag
			fallthrough
		default:
			switch vex := values[i].(type) {
			case map[string]pgtype.Text:
				for k, v := range vex {
					// we need to check if the key already exists. if it does, then don't overwrite it
					if _, ok := tags[k]; !ok {
						tags[k] = v.String
					}
				}
			case *pgtype.Numeric:
				var num float64
				vex.AssignTo(&num)

				tags[desc.Name] = num
			default:
				value, err := transformVal(desc.DataType, values[i])
				if err != nil {
					return gid, geom, tags, fmt.Errorf("unable to convert field [%v] (%v) of type (%v - %v) to a suitable value: %+v", i, desc.Name, desc.DataType, desc.DataTypeName, values[i])
				}

				tags[desc.Name] = value
			}
		}
	}

	return gid, geom, tags, nil
}

func gId(v interface{}) (gid uint64, err error) {
	switch aval := v.(type) {
	case float64:
		return uint64(aval), nil
	case int64:
		return uint64(aval), nil
	case uint64:
		return aval, nil
	case uint:
		return uint64(aval), nil
	case int8:
		return uint64(aval), nil
	case uint8:
		return uint64(aval), nil
	case uint16:
		return uint64(aval), nil
	case int32:
		return uint64(aval), nil
	case uint32:
		return uint64(aval), nil
	case string:
		return strconv.ParseUint(aval, 10, 64)
	default:
		return gid, fmt.Errorf("unable to convert field into a uint64.")
	}
}
