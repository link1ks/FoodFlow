package app

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// completeMeal is the authoritative cooking write. The meal completion row,
// FEFO batch decrements, outbound ledger, and replenishment suggestions commit
// together. A second request for the same household/day/meal cannot deduct.
func (a *App) completeMeal(c *gin.Context) {
	if !writable(c) {
		return
	}
	request := completeMealRequest{c.Param("plan"), c.Param("meal")}
	a.idem(c, request, func(tx pgx.Tx) (any, error) {
		return a.completeMealTx(c.Request.Context(), tx, cookingScope{HouseholdID: hid(c), ActorID: uid(c)}, request)
	})
}
