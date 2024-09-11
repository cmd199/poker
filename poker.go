package main

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type Request struct {
	Hands []string `json:"hands"`
}

type Hand struct {
	RequestId     string `json:"requestId"`
	Hand          string `json:"hand"`
	EvaluatedHand string `json:"yaku"`
	Strongest     bool   `json:"strongest"`
	Cards         []Card `json:"-"`
	Point         int    `json:"-"`
	StrongestRank int    `json:"-"`
}

type ErrorHand struct {
	RequestId    string `json:"requestId"`
	Hand         string `json:"hand"`
	ErrorMessage string `json:"errorMessage"`
}

type Response struct {
	Results []Hand      `json:"results"`
	Errors  []ErrorHand `json:"errors"`
}

type Card struct {
	Suit string
	Rank int
}

const (
	InvalidFormat     = "不正なフォーマットです"
	InvalidHandLength = "手札は5枚入力してください"
	InvalidCard       = "不正なカードが含まれています"
	InvalidSameRank   = "同じランクのカードは最大で4枚までです"
	InvalidSameCards  = "同じカードを2回以上入力しています"
)

func main() {
	e := echo.New()
	e.POST("/", hdl)
	e.Logger.Fatal(e.Start(":1323"))
}

func hdl(c echo.Context) error {
	var err error
	var error_hands []ErrorHand
	var strongest_point int
	var index_strongest_hands []int
	var strongest_rank []int
	var correct_hand []Hand

	req := new(Request)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": InvalidFormat})
	}

	// 役の判定
	for i := 0; i < len(req.Hands); i++ {
		hand := getHand(*req, i)
		hand.EvaluatedHand, err = evaluateHand(hand.Cards)
		if err != nil {
			error_hands = append(error_hands, ErrorHand{
				RequestId:    hand.RequestId,
				Hand:         hand.Hand,
				ErrorMessage: err.Error(),
			})
			continue
		}

		hand.Point = givePoint(hand.EvaluatedHand)

		// 最も強い役のインデックスを収集
		if hand.Point == strongest_point {
			index_strongest_hands = append(index_strongest_hands, len(correct_hand))
			strongest_rank = append(strongest_rank, getStrongestRank(hand.Cards, hand.Point))
		} else if strongest_point < hand.Point {
			strongest_point = hand.Point
			index_strongest_hands = []int{len(correct_hand)}
			strongest_rank = []int{getStrongestRank(hand.Cards, hand.Point)}
		}

		correct_hand = append(correct_hand, hand)
	}

	// 強さ判定
	for i := 0; i < len(index_strongest_hands); i++ {
		hand_index := index_strongest_hands[i]
		if strongest_rank[i] == slices.Max(strongest_rank) {
			correct_hand[hand_index].Strongest = true
		} else {
			correct_hand[hand_index].Strongest = false
		}
	}

	return c.JSON(http.StatusOK, Response{
		Results: correct_hand,
		Errors:  error_hands,
	})
}

func getId(i int) string {
	return fmt.Sprintf("01-00002-%02d", i+1)
}

func getCards(hand Hand) []Card {
	cards := strings.Split(hand.Hand, ", ")
	hand.Cards = make([]Card, len(cards))
	for j, card := range cards {
		if card != "" {
			hand.Cards[j].Suit = string(card[0])
			hand.Cards[j].Rank, _ = strconv.Atoi(card[1:])
		}
	}
	return hand.Cards
}

func getHand(req Request, i int) Hand {
	hand := Hand{
		RequestId: getId(i),
		Hand:      req.Hands[i],
	}
	hand.Cards = getCards(hand)
	return hand
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

func evaluateHand(cards []Card) (string, error) {

	if len(cards) != 5 {
		return "", errors.New(InvalidHandLength)
	}
	if checkDuplication(cards) {
		return "", errors.New(InvalidSameCards)
	}

	suits := getSuits(cards)
	ranks := getRanks(cards)

	for i := 0; i < len(cards); i++ {
		if suits[i] == "" {
			return "", errors.New(InvalidHandLength)
		}
		if !(suits[i] == "s" || suits[i] == "k" || suits[i] == "d" || suits[i] == "h") {
			return "", errors.New(InvalidCard)
		}
		if !(1 <= ranks[i] && ranks[i] <= 13) {
			return "", errors.New(InvalidCard)
		}
	}

	unique_ranks := makeUniqueRanks(ranks)
	grouped_ranks := groupRanks(ranks)

	switch len(unique_ranks) {
	case 5:
		if isRoyalStraightFlush(suits, ranks) {
			return "ロイヤルストレートフラッシュ", nil
		} else if isStraightFlush(suits, ranks) {
			return "ストレートフラッシュ", nil
		} else if isSingleSuits(suits) {
			return "フラッシュ", nil
		} else if isStraight(ranks) || isRoyalStraight(ranks) {
			return "ストレート", nil
		} else {
			return "ハイカード", nil
		}
	case 4:
		return "ワンペア", nil
	case 3:
		if len(grouped_ranks[0]) == 3 || len(grouped_ranks[1]) == 3 || len(grouped_ranks[2]) == 3 {
			return "スリーカード", nil
		} else if len(grouped_ranks[0]) == 2 || len(grouped_ranks[1]) == 2 || len(grouped_ranks[2]) == 2 {
			return "ツーペア", nil
		}
	case 2:
		if len(grouped_ranks[0]) == 4 || len(grouped_ranks[1]) == 4 {
			return "フォーカード", nil
		} else if len(grouped_ranks[0]) == 3 || len(grouped_ranks[1]) == 3 {
			return "フルハウス", nil
		}
	case 1:
		return "", errors.New(InvalidSameRank)
	}

	return "ハイカード", nil
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

// func getStrongestIndex(hand Hand) ([]int, []int) {
// 	var strongest_point int
// 	var index_strongest_hands []int
// 	var strongest_rank []int
// 	var correct_hand []int

// 	if hand.Point == strongest_point {
// 		index_strongest_hands = append(index_strongest_hands, len(correct_hand))
// 		strongest_rank = append(strongest_rank, getStrongestRank(hand.Cards, hand.Point))
// 	} else if strongest_point < hand.Point {
// 		strongest_point = hand.Point
// 		index_strongest_hands = []int{len(correct_hand)}
// 		strongest_rank = []int{getStrongestRank(hand.Cards, hand.Point)}
// 	}
// 	return index_strongest_hands, strongest_rank
// }

func getStrongestRank(cards []Card, strongest_point int) int {
	var strongest_rank int
	ranks := getRanks(cards)
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

func checkDuplication(cards []Card) bool {
	m := make(map[Card]bool)
	for _, card := range cards {
		if m[card] {
			return true
		}
		m[card] = true
	}

	return false
}
