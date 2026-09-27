-- +goose Up
CREATE TABLE ingredient_catalog (
    id uuid PRIMARY KEY,
    name text NOT NULL UNIQUE,
    category text NOT NULL,
    default_unit text NOT NULL,
    aliases text[] NOT NULL DEFAULT '{}'
);
ALTER TABLE ingredients ADD COLUMN catalog_id uuid REFERENCES ingredient_catalog(id);

INSERT INTO ingredient_catalog(id,name,category,default_unit,aliases)
SELECT md5('foodflow:ingredient:' || name)::uuid,name,category,default_unit,aliases
FROM (VALUES
    ('番茄','蔬菜','g',ARRAY['西红柿']),('黄瓜','蔬菜','g',ARRAY[]::text[]),('土豆','蔬菜','g',ARRAY['马铃薯']),
    ('胡萝卜','蔬菜','g',ARRAY[]::text[]),('白萝卜','蔬菜','g',ARRAY[]::text[]),('洋葱','蔬菜','g',ARRAY[]::text[]),
    ('西兰花','蔬菜','g',ARRAY[]::text[]),('花菜','蔬菜','g',ARRAY['菜花']),('生菜','蔬菜','g',ARRAY[]::text[]),
    ('菠菜','蔬菜','g',ARRAY[]::text[]),('小白菜','蔬菜','g',ARRAY[]::text[]),('大白菜','蔬菜','g',ARRAY[]::text[]),
    ('卷心菜','蔬菜','g',ARRAY['包菜']),('油麦菜','蔬菜','g',ARRAY[]::text[]),('芹菜','蔬菜','g',ARRAY[]::text[]),
    ('茄子','蔬菜','g',ARRAY[]::text[]),('青椒','蔬菜','g',ARRAY[]::text[]),('红椒','蔬菜','g',ARRAY[]::text[]),
    ('南瓜','蔬菜','g',ARRAY[]::text[]),('冬瓜','蔬菜','g',ARRAY[]::text[]),('丝瓜','蔬菜','g',ARRAY[]::text[]),
    ('玉米','蔬菜','g',ARRAY[]::text[]),('豌豆','蔬菜','g',ARRAY[]::text[]),('四季豆','蔬菜','g',ARRAY[]::text[]),
    ('莲藕','蔬菜','g',ARRAY[]::text[]),('韭菜','蔬菜','g',ARRAY[]::text[]),('香菜','蔬菜','g',ARRAY[]::text[]),
    ('苹果','水果','g',ARRAY[]::text[]),('香蕉','水果','g',ARRAY[]::text[]),('橙子','水果','g',ARRAY[]::text[]),
    ('梨','水果','g',ARRAY[]::text[]),('葡萄','水果','g',ARRAY[]::text[]),('草莓','水果','g',ARRAY[]::text[]),
    ('蓝莓','水果','g',ARRAY[]::text[]),('西瓜','水果','g',ARRAY[]::text[]),('柠檬','水果','g',ARRAY[]::text[]),
    ('牛肉','肉禽','g',ARRAY[]::text[]),('猪肉','肉禽','g',ARRAY[]::text[]),('鸡肉','肉禽','g',ARRAY[]::text[]),
    ('鸡胸肉','肉禽','g',ARRAY[]::text[]),('鸡腿','肉禽','g',ARRAY[]::text[]),('排骨','肉禽','g',ARRAY[]::text[]),
    ('羊肉','肉禽','g',ARRAY[]::text[]),('鸭肉','肉禽','g',ARRAY[]::text[]),
    ('虾','水产','g',ARRAY[]::text[]),('鱼肉','水产','g',ARRAY[]::text[]),('三文鱼','水产','g',ARRAY[]::text[]),
    ('带鱼','水产','g',ARRAY[]::text[]),('贝类','水产','g',ARRAY[]::text[]),
    ('鸡蛋','蛋奶','个',ARRAY[]::text[]),('鸭蛋','蛋奶','个',ARRAY[]::text[]),('牛奶','蛋奶','ml',ARRAY[]::text[]),
    ('酸奶','蛋奶','g',ARRAY[]::text[]),('奶酪','蛋奶','g',ARRAY[]::text[]),
    ('豆腐','豆制品','g',ARRAY[]::text[]),('豆干','豆制品','g',ARRAY[]::text[]),('豆皮','豆制品','g',ARRAY[]::text[]),
    ('豆浆','豆制品','ml',ARRAY[]::text[]),('黄豆','豆制品','g',ARRAY[]::text[]),
    ('大米','主食谷物','g',ARRAY[]::text[]),('面粉','主食谷物','g',ARRAY[]::text[]),('面条','主食谷物','g',ARRAY[]::text[]),
    ('米粉','主食谷物','g',ARRAY[]::text[]),('燕麦','主食谷物','g',ARRAY[]::text[]),('面包','主食谷物','g',ARRAY[]::text[]),
    ('馒头','主食谷物','个',ARRAY[]::text[]),('红薯','主食谷物','g',ARRAY['地瓜']),
    ('香菇','菌菇','g',ARRAY[]::text[]),('口蘑','菌菇','g',ARRAY[]::text[]),('金针菇','菌菇','g',ARRAY[]::text[]),
    ('木耳','菌菇','g',ARRAY[]::text[]),('杏鲍菇','菌菇','g',ARRAY[]::text[]),
    ('蒜','调味香料','g',ARRAY['大蒜']),('姜','调味香料','g',ARRAY['生姜']),('葱','调味香料','g',ARRAY['小葱']),
    ('食盐','调味香料','g',ARRAY['盐']),('白糖','调味香料','g',ARRAY['糖']),('食用油','调味香料','ml',ARRAY[]::text[]),
    ('酱油','调味香料','ml',ARRAY['生抽']),('醋','调味香料','ml',ARRAY[]::text[]),('黑胡椒','调味香料','g',ARRAY[]::text[]),
    ('蜂蜜','调味香料','g',ARRAY[]::text[]),('蚝油','调味香料','ml',ARRAY[]::text[]),
    ('饮用水','饮品','ml',ARRAY['水']),('椰奶','饮品','ml',ARRAY[]::text[]),('咖啡','饮品','ml',ARRAY[]::text[])
) AS seed(name,category,default_unit,aliases);

UPDATE ingredients i SET catalog_id=c.id
FROM ingredient_catalog c WHERE i.name=c.name AND i.unit=c.default_unit;

-- +goose Down
ALTER TABLE ingredients DROP COLUMN catalog_id;
DROP TABLE ingredient_catalog;
