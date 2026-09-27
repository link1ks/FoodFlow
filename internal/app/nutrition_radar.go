package app

import (
	"encoding/json"
	"foodflow/internal/dbgen"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func (a *App) nutritionRadar(c *gin.Context) {
	days := int32(7)
	switch c.DefaultQuery("days", "7") {
	case "7":
	case "30":
		days = 30
	default:
		fail(c, 400, "仅支持近 7 天或 30 天")
		return
	}
	var house pgtype.UUID
	if err := house.Scan(hid(c)); err != nil {
		fail(c, 400, "家庭无效")
		return
	}
	report, err := dbgen.New(a.DB).GetNutritionRadar(c, dbgen.GetNutritionRadarParams{HouseholdID: house, PeriodDays: days})
	if err != nil {
		fail(c, 500, "营养记录读取失败")
		return
	}
	c.Data(200, "application/json; charset=utf-8", json.RawMessage(report))
}
