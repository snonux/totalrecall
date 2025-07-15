package image

import "strings"

// translateBulgarianQuery attempts to translate a Bulgarian query to English for better results
// This is a simple implementation - in production you might use a translation API
func translateBulgarianQuery(query string) string {
	// Common Bulgarian words for flashcard creation
	translations := map[string]string{
		"ябълка": "apple",
		"малинка": "raspberry",
		"ягода":  "strawberry",
		"череша": "cherry",
		"круша":  "pear",
		"праскова": "peach",
		"грозде": "grapes",
		"банан":  "banana",
		"портокал": "orange",
		"лимон":  "lemon",
		"котка":  "cat",
		"куче":   "dog",
		"хляб":   "bread",
		"вода":   "water",
		"къща":   "house",
		"дърво":  "tree",
		"цвете":  "flower",
		"книга":  "book",
		"стол":   "chair",
		"маса":   "table",
		"прозорец": "window",
		"врата":  "door",
		"ръка":   "hand",
		"око":    "eye",
		"слънце": "sun",
		"луна":   "moon",
		"звезда": "star",
		"море":   "sea",
		"планина": "mountain",
		"кола":   "car",
		"автобус": "bus",
		"влак":   "train",
		"самолет": "airplane",
		"училище": "school",
		"учител": "teacher",
		"ученик": "student",
		"приятел": "friend",
		"семейство": "family",
		"майка":  "mother",
		"баща":   "father",
		"брат":   "brother",
		"сестра": "sister",
		"дете":   "child",
		"мъж":    "man",
		"жена":   "woman",
		"момче":  "boy",
		"момиче": "girl",
		"храна":  "food",
		"плод":   "fruit",
		"зеленчук": "vegetable",
		"мляко":  "milk",
		"сирене": "cheese",
		"месо":   "meat",
		"риба":   "fish",
		"пиле":   "chicken",
		"яйце":   "egg",
		"захар":  "sugar",
		"сол":    "salt",
		"кафе":   "coffee",
		"чай":    "tea",
		"вино":   "wine",
		"бира":   "beer",
		"сок":    "juice",
		"град":   "city",
		"село":   "village",
		"улица":  "street",
		"парк":   "park",
		"магазин": "shop",
		"ресторант": "restaurant",
		"хотел":  "hotel",
		"болница": "hospital",
		"аптека": "pharmacy",
		"банка":  "bank",
		"пощa":   "post office",
		"полиция": "police",
		"пожарна": "fire station",
		"летище": "airport",
		"гара":   "train station",
	}
	
	// Try exact match first
	query = strings.ToLower(strings.TrimSpace(query))
	if translated, ok := translations[query]; ok {
		return translated
	}
	
	// If no translation found, return original
	// Pixabay might still return results for common words
	return query
}