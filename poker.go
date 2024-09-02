package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type Input struct {
	Hands []string `json:"hands"`
}

type Hand struct {
	RequetID      string `json:"requestId"`
	Hand          string `json:"hand"`
	Cards         []Card `json:"-"`
	EvaluatedHand string `json:"yaku"`
	Point         int    `json:"-"`
	Strongest     bool   `json:"strongest"`
	StrongestRank int    `json:"-"`
}

type Card struct {
	Suit string
	Rank int
}

func main() {
	e := echo.New()
	e.POST("/", hdl)
	e.Logger.Fatal(e.Start(":1323"))
}

func hdl(c echo.Context) error {

	// リクエストボディの読み取り
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "不正なフォーマットです。",
		})
	}

	// JSONを構造体にデコードする
	var hands_from_json Input
	if err := json.Unmarshal(body, &hands_from_json); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"message": "不正なフォーマットです。",
		})
	}

	// 手札の受け取り処理
	hand := make([]Hand, len(hands_from_json.Hands))
	for i := 0; i < len(hand); i++ {
		// IDの付与
		hand[i].RequetID = fmt.Sprintf("01-00002-%02d", i+1)

		// 手札の受け取り
		hand[i].Hand = hands_from_json.Hands[i]

		// 手札をカード配列に分割
		cards := strings.Split(hand[i].Hand, ", ")

		// スーツとランクの受け取り
		hand[i].Cards = make([]Card, len(cards))
		for j, card := range cards {
			suit_rank := strings.SplitN(card, "", 2)
			hand[i].Cards[j].Suit = suit_rank[0]
			hand[i].Cards[j].Rank, _ = strconv.Atoi(suit_rank[1])
		}
	}

	// 役判定処理
	var strongest_point int
	var index_strongest_hands []int
	var strongest_rank []int
	for i := 0; i < len(hand); i++ {
		// 役判定
		hand[i].EvaluatedHand = evaluateCards(hand[i].Cards)
		hand[i].Point = givePoint(hand[i].EvaluatedHand)

		// 最も強い役のインデックスを収集
		if hand[i].Point == strongest_point {
			index_strongest_hands = append(index_strongest_hands, i)
			strongest_rank = append(strongest_rank, getStrongestRank(getRanks(hand[i].Cards), hand[i].Point))
		} else if strongest_point < hand[i].Point {
			strongest_point = hand[i].Point
			index_strongest_hands = []int{i}
			strongest_rank = []int{getStrongestRank(getRanks(hand[i].Cards), hand[i].Point)}
		}
	}

	// 強さ判定処理
	for i := 0; i < len(index_strongest_hands); i++ {
		hand_index := index_strongest_hands[i]
		if strongest_rank[i] == slices.Max(strongest_rank) {
			hand[hand_index].Strongest = true
		} else {
			hand[hand_index].Strongest = false
		}
	}

	// 構造体をJSONにエンコードする
	results, err := json.Marshal(hand)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", results)
	return c.JSON(http.StatusOK, map[string][]Hand{
		"results": hand,
	})
}

func getSuits(cards []Card) []string {
	suits := make([]string, len(cards))
	for i := 0; i < len(cards); i++ {
		suits[i] = cards[i].Suit
	}
	slices.Sort(suits)
	return suits
}

func getRanks(cards []Card) []int {
	ranks := make([]int, len(cards))
	for i := 0; i < len(cards); i++ {
		ranks[i] = cards[i].Rank
	}
	slices.Sort(ranks)
	return ranks
}

func isSingleSuits(suits []string) bool {
	copy_suits := make([]string, len(suits))
	copy(copy_suits, suits)
	unique_suits := slices.Compact(copy_suits)

	return len(unique_suits) == 1
}

func makeUniqueRanks(ranks []int) []int {
	copy_ranks := make([]int, len(ranks))
	copy(copy_ranks, ranks)
	unique_ranks := slices.Compact(copy_ranks)
	return unique_ranks
}

func groupRanks(ranks []int) [][]int {
	unique_ranks := makeUniqueRanks(ranks)
	grouped_ranks := make([][]int, len(makeUniqueRanks(ranks)))

	for i := 0; i < len(unique_ranks); i++ {
		for j := 0; j < len(ranks); j++ {
			if unique_ranks[i] == ranks[j] {
				grouped_ranks[i] = append(grouped_ranks[i], unique_ranks[i])
			}
		}
	}
	return grouped_ranks
}

func evaluateCards(cards []Card) string {
	suits := getSuits(cards)
	ranks := getRanks(cards)
	unique_ranks := makeUniqueRanks(ranks)
	grouped_ranks := groupRanks(ranks)

	switch len(unique_ranks) {
	case 5:
		if isRoyalStraightFlush(suits, ranks) {
			return "ロイヤルストレートフラッシュ"
		} else if isStraightFlush(suits, ranks) {
			return "ストレートフラッシュ"
		} else if isSingleSuits(suits) {
			return "フラッシュ"
		} else if isStraight(ranks) || isRoyalStraight(ranks) {
			return "ストレート"
		} else {
			return "ハイカード"
		}
	case 4:
		return "ワンペア"
	case 3:
		if len(grouped_ranks[0]) == 3 || len(grouped_ranks[1]) == 3 || len(grouped_ranks[2]) == 3 {
			return "スリーカード"
		} else if len(grouped_ranks[0]) == 2 || len(grouped_ranks[1]) == 2 || len(grouped_ranks[2]) == 2 {
			return "ツーペア"
		}
	case 2:
		if len(grouped_ranks[0]) == 4 || len(grouped_ranks[1]) == 4 {
			return "フォーカード"
		} else if len(grouped_ranks[0]) == 3 || len(grouped_ranks[1]) == 3 {
			return "フルハウス"
		}
	}
	return "ハイカード"
}

func isRoyalStraightFlush(suits []string, ranks []int) bool {
	is_flush := isSingleSuits(suits)
	is_royal_straight := isRoyalStraight(ranks)

	if is_flush && is_royal_straight {
		return true
	}
	return false
}

func isStraightFlush(suits []string, ranks []int) bool {
	is_flush := isSingleSuits(suits)
	is_straight := isStraight(ranks)

	if is_flush && is_straight {
		return true
	}
	return false
}

func isStraight(ranks []int) bool {
	unique_ranks := makeUniqueRanks(ranks)
	is_straight := false

	if slices.Max(unique_ranks)-slices.Min(unique_ranks) == 4 {
		is_straight = true
	}
	return is_straight
}

func isRoyalStraight(ranks []int) bool {
	unique_ranks := makeUniqueRanks(ranks)
	royal_straight := []int{1, 10, 11, 12, 13}
	is_royal_straight := false

	if slices.Equal(unique_ranks, royal_straight) {
		is_royal_straight = true
	}
	return is_royal_straight
}

func givePoint(evaluated_hand string) int {
	switch evaluated_hand {
	case "ロイヤルストレートフラッシュ":
		return 10
	case "ストレートフラッシュ":
		return 9
	case "フォーカード":
		return 8
	case "フルハウス":
		return 7
	case "フラッシュ":
		return 6
	case "ストレート":
		return 5
	case "スリーカード":
		return 4
	case "ツーペア":
		return 3
	case "ワンペア":
		return 2
	case "ハイカード":
		return 1
	}
	return 1
}

func getStrongestRank(ranks []int, strongest_point int) int {
	var strongest_rank int
	grouped_ranks := groupRanks(ranks)

	switch strongest_point {

	case 2:
		for i := 0; i < len(grouped_ranks); i++ {
			if len(grouped_ranks[i]) == 2 {
				if grouped_ranks[i][0] == 1 {
					strongest_rank = 14
				} else if strongest_rank <= grouped_ranks[i][0] {
					strongest_rank = grouped_ranks[i][0]
				}
			}
		}

	case 3:
		for i := 0; i < len(grouped_ranks); i++ {
			if len(grouped_ranks[i]) == 2 {
				if grouped_ranks[i][0] == 1 {
					strongest_rank = 14
				} else if strongest_rank <= grouped_ranks[i][0] {
					strongest_rank = grouped_ranks[i][0]
				}
			}
		}

	case 4:
		for i := 0; i < len(grouped_ranks); i++ {
			if len(grouped_ranks[i]) == 3 {
				if grouped_ranks[i][0] == 1 {
					strongest_rank = 14
				} else if strongest_rank <= grouped_ranks[i][0] {
					strongest_rank = grouped_ranks[i][0]
				}
			}
		}

	case 5:
		for i := 0; i < len(ranks); i++ {
			if isRoyalStraight(ranks) {
				strongest_rank = 14
			} else if strongest_rank <= ranks[i] {
				strongest_rank = ranks[i]
			}
		}

	case 6:
		for i := 0; i < len(ranks); i++ {
			if ranks[i] == 1 {
				strongest_rank = 14
			} else if strongest_rank <= ranks[i] {
				strongest_rank = ranks[i]
			}
		}

	case 7:
		for i := 0; i < len(grouped_ranks); i++ {
			if len(grouped_ranks[i]) == 3 {
				if grouped_ranks[i][0] == 1 {
					strongest_rank = 14
				} else if strongest_rank <= grouped_ranks[i][0] {
					strongest_rank = grouped_ranks[i][0]
				}
			}
		}

	case 8:
		for i := 0; i < len(grouped_ranks); i++ {
			if len(grouped_ranks[i]) == 4 {
				if grouped_ranks[i][0] == 1 {
					strongest_rank = 14
				} else if strongest_rank <= grouped_ranks[i][0] {
					strongest_rank = grouped_ranks[i][0]
				}
			}
		}

	case 9:
		for i := 0; i < len(ranks); i++ {
			if isRoyalStraight(ranks) {
				strongest_rank = 14
			} else if strongest_rank <= ranks[i] {
				strongest_rank = ranks[i]
			}
		}

	case 1:
		for i := 0; i < len(ranks); i++ {
			if ranks[i] == 1 {
				strongest_rank = 14
			} else if strongest_rank <= ranks[i] {
				strongest_rank = ranks[i]
			}
		}
	}

	return strongest_rank
}
