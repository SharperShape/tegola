package mvt

import (
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
)

/*
func TestEncodeGeometry(t *testing.T) {
	testcases := []struct {
		geo  tegola.Geometry
		typ  vectorTile.Tile_GeomType
		egeo []uint32
		eerr error
	}{
		{
			geo:  nil,
			typ:  vectorTile.Tile_UNKNOWN,
			egeo: []uint32{},
			eerr: ErrUnknownGeometryType,
		},
		{
			geo:  &basic.Point{1, 1},
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{9, 2, 2},
		},
		{
			geo:  &basic.Point{25, 17},
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{9, 50, 34},
		},
		{
			geo:  &basic.MultiPoint{basic.Point{5, 7}, basic.Point{3, 2}},
			typ:  vectorTile.Tile_POINT,
			egeo: []uint32{17, 10, 14, 3, 9},
		},
		{
			geo:  &basic.Line{basic.Point{2, 2}, basic.Point{2, 10}, basic.Point{10, 10}},
			typ:  vectorTile.Tile_LINESTRING,
			egeo: []uint32{9, 4, 4, 18, 0, 16, 16, 0},
		},
		{
			geo: &basic.MultiLine{
				basic.Line{basic.Point{2, 2}, basic.Point{2, 10}, basic.Point{10, 10}},
				basic.Line{basic.Point{1, 1}, basic.Point{3, 5}},
			},
			typ:  vectorTile.Tile_LINESTRING,
			egeo: []uint32{9, 4, 4, 18, 0, 16, 16, 0, 9, 17, 17, 10, 4, 8},
		},
		{
			geo: &basic.Polygon{
				basic.Line{
					basic.Point{3, 6},
					basic.Point{8, 12},
					basic.Point{20, 34},
				},
			},
			typ:  vectorTile.Tile_POLYGON,
			egeo: []uint32{9, 6, 12, 18, 10, 12, 24, 44, 15},
		},
		{
			geo: &basic.MultiPolygon{
				basic.Polygon{
					basic.Line{
						basic.Point{0, 0},
						basic.Point{10, 0},
						basic.Point{10, 10},
						basic.Point{0, 10},
					},
				},
				basic.Polygon{
					basic.Line{
						basic.Point{11, 11},
						basic.Point{20, 11},
						basic.Point{20, 20},
						basic.Point{11, 20},
					},
					basic.Line{
						basic.Point{13, 13},
						basic.Point{13, 17},
						basic.Point{17, 17},
						basic.Point{17, 13},
					},
				},
			},
			typ:  vectorTile.Tile_POLYGON,
			egeo: []uint32{9, 0, 0, 26, 20, 0, 0, 20, 19, 0, 15, 9, 22, 2, 26, 18, 0, 0, 18, 17, 0, 15, 9, 4, 13, 26, 0, 8, 8, 0, 0, 7, 15},
		},
	}
	for _, tcase := range testcases {
		g, gtype, err := encodeGeometry(tcase.geo, 0, 0, 4096)
		if tcase.eerr != err {
			t.Errorf("Expected error (%v) got (%v) instead", tcase.eerr, err)
		}
		if gtype != tcase.typ {
			t.Errorf("Expected Geometry Type to be %v Got: %v", tcase.typ, gtype)
		}
		if len(g) != len(tcase.egeo) {
			t.Errorf("Geometry length is not what was expected(%v) got (%v)", tcase.egeo, g)
			continue
		}
		for i := range tcase.egeo {
			if tcase.egeo[i] != g[i] {
				t.Errorf("Geometry is not what was expected(%v) got (%v)", tcase.egeo, g)
				break
			}
		}
	}
}

func TestNewFeature(t *testing.T) {
	testcases := []struct {
		geo      tegola.Geometry
		tags     map[string]interface{}
		expected []Feature
	}{
		{
			geo:      nil,
			tags:     nil,
			expected: []Feature{},
		},
	}
	for i, tcase := range testcases {
		got := NewFeatures(tcase.geo, tcase.tags)
		if len(tcase.expected) != len(got) {
			t.Errorf("Test %v: Expected to get %v features got %v features.", i, len(tcase.expected), len(got))
			continue
		}
		if len(tcase.expected) <= 0 {
			continue
		}
		// TOOD test to make sure we got the correct feature

	}
}
*/

func TestNormalizePoint(t *testing.T) {
	/*
	   fn test_point_to_screen_coords() {
	       //let zh_mercator = geom::Point::new(949398.0, 6002729.0);
	       let zh_mercator = geom::Point::new(960000.0, 6002729.0);
	       //let zh_wgs84 = postgis::Point::<WGS84>::new(47.3703149, 8.5285874);
	       let tile_extent = Extent {minx: 958826.08, miny: 5987771.04, maxx: 978393.96, maxy: 6007338.92};
	       let screen_pt = screen::Point::from_geom(&tile_extent, false, 4096, &zh_mercator);
	       assert_eq!(screen_pt, screen::Point { x: 245, y: 3131 });
	       assert_eq!(screen_pt.encode().vec(), &[9,490,6262]);
	   }
	*/

	testcases := []struct {
		point  basic.Point
		extent tegola.Extent
		nx, ny float64
	}{
		{
			point: basic.Point{960000, 6002729},
			extent: tegola.Extent{
				Minx: 958826.08,
				Miny: 5987771.04,
				Maxx: 978393.96,
				Maxy: 6007338.92,
			},
			nx: 245,
			ny: 3131,
		},
	}

	for i, tcase := range testcases {
		c := cursor{
			X:      0,
			Y:      0,
			extent: 4096,
		}
		nx, ny := c.NormalizePoint(tcase.extent, &tcase.point)

		if nx != tcase.nx {
			t.Errorf("Test %v: Expected nx value of %v got %v.", i, tcase.nx, nx)
		}
		if ny != tcase.ny {
			t.Errorf("Test %v: Expected ny value of %v got %v.", i, tcase.ny, ny)
		}
		continue
	}
}
