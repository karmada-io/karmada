package gronx

import (
	"strings"
	"time"
)

// Expr represents an item in array for batch check
type Expr struct {
	Expr string
	Due  bool
	Err  error
}

// BatchDue checks if multiple expressions are due for given time (or now).
// It returns []Expr with filled in Due and Err values.
func (g *Gronx) BatchDue(exprs []string, ref ...time.Time) []Expr {
	ref = append(ref, time.Now())
	g.C.SetRef(ref[0])

	var segs []string

	cache, batch := map[string]Expr{}, make([]Expr, len(exprs))
	for i := range exprs {
		batch[i].Expr = exprs[i]
		segs, batch[i].Err = Segments(exprs[i])
		key := strings.Join(segs, " ")
		if batch[i].Err != nil {
			cache[key] = batch[i]
			continue
		}

		if c, ok := cache[key]; ok {
			batch[i] = c
			batch[i].Expr = exprs[i]
			continue
		}

		due := true
		for pos, seg := range segs {
			if seg != "*" && seg != "?" {
				if due, batch[i].Err = g.C.CheckDue(seg, pos); !due || batch[i].Err != nil {
					break
				}
			}
		}
		batch[i].Due = due
		cache[key] = batch[i]
	}
	return batch
}
