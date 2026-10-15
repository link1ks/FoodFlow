package intake

import "testing"

func TestConservativeSpeechDrafts(t *testing.T) {
	catalog := []CatalogItem{{"tomato", "番茄", "g", []string{"西红柿"}}, {"egg", "鸡蛋", "个", nil}, {"milk", "牛奶", "ml", nil}, {"potato", "土豆", "g", nil}}
	for _, tc := range []struct {
		text       string
		quantities []string
	}{
		{"西红柿半斤，鸡蛋六个，牛奶一瓶。", []string{"250.000", "6.000", ""}},
		{"番茄一斤半", []string{""}},
		{"2.5公斤番茄、牛奶零点五升", []string{"2500.000", "500.000"}},
		{"买了土豆一点五千克。", []string{"1500.000"}},
		{"土豆零点五千克", []string{"500.000"}},
		{"土豆一千点五千克", []string{"1000500.000"}},
		{"土豆一点五百克", []string{""}},
		{"土豆一点五六七八千克", []string{""}},
		{"番茄三百二十克", []string{"320.000"}},
		{"番茄二三克", []string{""}},
		{"番茄三百百克", []string{""}},
		{"500克番茄300克", []string{""}},
		{"番茄500克土豆", []string{"500.000", ""}},
		{"番茄和六个鸡蛋", []string{"", "6.000"}},
		{"番茄-2克", []string{""}},
		{"番茄0.0001克", []string{""}},
		{"番茄99999999999999999999斤", []string{""}},
		{"番茄大概500克", []string{""}},
		{"番茄500克不对300克", []string{""}},
		{"不要番茄500克", []string{""}},
		{"牛奶500克", []string{""}},
		{"没有目录名称200克", []string{}},
	} {
		t.Run(tc.text, func(t *testing.T) {
			got := Parse(tc.text, catalog)
			if len(got.Items) != len(tc.quantities) {
				t.Fatalf("items=%+v", got.Items)
			}
			for i, q := range tc.quantities {
				if got.Items[i].Quantity != q {
					t.Fatalf("item %d=%+v want %s", i, got.Items[i], q)
				}
				if q == "" && got.Items[i].Warning == "" {
					t.Fatal("missing warning")
				}
			}
		})
	}
	ambiguous := Parse("番茄500克", append(catalog, CatalogItem{"other", "小番茄", "g", []string{"番茄"}}))
	if ambiguous.Items[0].CatalogID != "" || ambiguous.Items[0].Quantity != "" {
		t.Fatal("ambiguous alias accepted")
	}
	longest := Parse("小番茄500克", append(catalog, CatalogItem{"small", "小番茄", "g", nil}))
	if len(longest.Items) != 1 || longest.Items[0].CatalogID != "small" {
		t.Fatal("overlapping names")
	}
}

func TestRetractionsAndCorrectionsNeverPrefillQuantities(t *testing.T) {
	catalog := []CatalogItem{{"tomato", "番茄", "g", nil}, {"egg", "鸡蛋", "个", nil}, {"milk", "牛奶", "ml", nil}}
	for _, text := range []string{
		"别加鸡蛋三个。",
		"添加牛奶二百五十毫升，取消牛奶。",
		"鸡蛋三个，不对，鸡蛋四个。",
		"番茄五百克，去掉番茄。",
		"鸡蛋三个，改成四个。",
		"鸡蛋三个。改为四个。",
		"番茄500克；撤回刚才的操作。",
		"番茄500克，鸡蛋大概三个。",
		"不用鸡蛋三个，番茄500克。",
		"删除番茄500克",
		"番茄500克，错了",
	} {
		t.Run(text, func(t *testing.T) {
			got := Parse(text, catalog)
			if len(got.Items) == 0 {
				t.Fatal("reviewable ingredient mentions lost")
			}
			for _, item := range got.Items {
				if item.Quantity != "" || item.Warning == "" {
					t.Fatalf("ambiguous utterance prefilled a quantity: %+v", item)
				}
			}
		})
	}
}
