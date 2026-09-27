package app

import "github.com/gin-gonic/gin"

// Archive only the shopping entry: purchased batches and ledger remain intact.
func (a *App) deleteShopping(c *gin.Context) {
	if !writable(c) {
		return
	}
	tag, err := a.DB.Exec(c, `UPDATE shopping_items i SET deleted_at=COALESCE(i.deleted_at,now()) FROM shopping_lists l WHERE i.list_id=l.id AND i.id=$1 AND l.household_id=$2`, c.Param("item"), hid(c))
	if err != nil {
		fail(c, 500, "delete failed")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(c, 404, "item not found")
		return
	}
	c.Status(204)
}
