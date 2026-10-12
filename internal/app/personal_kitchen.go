package app

import (
	"errors"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// Idempotent provisioning is explicit PUT, not a side effect of listing spaces.
// The user lock is also used by account merging, so concurrent sessions cannot
// create two default kitchens or race a credential-verified merge.
func (a *App) personalKitchen(c *gin.Context) {
	tx, err := a.DB.Begin(c)
	if err != nil {
		fail(c, 503, "个人厨房暂时无法加载，请重试")
		return
	}
	defer tx.Rollback(c)
	var user string
	if err = tx.QueryRow(c, "SELECT id FROM users WHERE id=$1 AND merged_into IS NULL FOR UPDATE", uid(c)).Scan(&user); err != nil {
		fail(c, 401, "请重新登录")
		return
	}
	var id, name string
	var servings int
	err = tx.QueryRow(c, `SELECT h.id,h.name,h.servings FROM personal_kitchens p
JOIN households h ON h.id=p.household_id WHERE p.user_id=$1 AND h.owner_id=$1 AND h.kind='personal'`, user).Scan(&id, &name, &servings)
	if errors.Is(err, pgx.ErrNoRows) {
		id, name, servings = core.ID(), "我的厨房", 1
		_, err = tx.Exec(c, `INSERT INTO households(id,name,owner_id,servings,kind,timezone)
VALUES($1,$2,$3,1,'personal',(SELECT timezone FROM users WHERE id=$3))`, id, name, user)
		if err == nil {
			_, err = tx.Exec(c, "INSERT INTO members(household_id,user_id,role) VALUES($1,$2,'owner')", id, user)
		}
		if err == nil {
			_, err = tx.Exec(c, "INSERT INTO personal_kitchens(user_id,household_id) VALUES($1,$2)", user, id)
		}
	}
	if err != nil || tx.Commit(c) != nil {
		fail(c, 503, "个人厨房暂时无法加载，请重试")
		return
	}
	c.JSON(200, gin.H{"id": id, "name": name, "servings": servings, "role": "owner", "kind": "personal"})
}

func sharedKitchen(c *gin.Context) bool {
	if c.GetString("kitchen_kind") == "personal" {
		fail(c, 403, "个人厨房仅自己可用；请在家庭共享中创建或加入家庭")
		return false
	}
	return true
}
