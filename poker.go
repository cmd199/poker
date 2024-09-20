package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	_ "github.com/lib/pq"
)

var Db *sql.DB

func init() {
	var err error

	// .envファイルから環境変数を読み込む
	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// 環境変数からデータベース接続情報を取得
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	// データベース接続文字列を作成
	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		host, port, user, dbname, password)

	// データベースに接続
	Db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}

	err = Db.Ping()
	if err != nil {
		fmt.Println(CONECTION_FAILURE)
		return
	} else {
		fmt.Println(CONNECTION_SUCCESSFUL)
	}

	// テーブル作成
	if err = CreateTabe(Db); err != nil {
		panic(err)
	}
}

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

type Error struct {
	RequestId    string `json:"requestId"`
	Hand         string `json:"hand"`
	ErrorMessage string `json:"errorMessage"`
}

type Response struct {
	Results []Hand  `json:"results"`
	Errors  []Error `json:"errors"`
}

type Card struct {
	Suit string
	Rank int
}

const (
	CONNECTION_SUCCESSFUL     = "接続成功"
	CONECTION_FAILURE         = "接続失敗"
	TABLE_CREATION_SUCCESSFUL = "テーブル作成成功"
	TABLE_CREATION_FAILURE    = "テーブル作成失敗"
	INTERNAL_SERVER_ERROR     = "サーバーでエラーが発生しています"

	INVALID_FORMAT      = "不正なフォーマットです"
	INVALID_HAND_LENGTH = "手札は5枚入力してください"
	INVALID_CARD        = "不正なカードが含まれています"
	INVALID_SAME_RANK   = "同じランクのカードは最大で4枚までです"
	INVALID_SAME_CARDS  = "同じカードを2回以上入力しています"

	ROYAL_STRAIGHT_FLUSH = "ロイヤルストレートフラッシュ"
	STRAIGHT_FLUSH       = "ストレートフラッシュ"
	FOUR_OF_A_KIND       = "フォーカード"
	FULL_HOUSE           = "フルハウス"
	FLUSH                = "フラッシュ"
	STRAIGHT             = "ストレート"
	THREE_OF_A_KIND      = "スリーカード"
	TWO_PAIR             = "ツーペア"
	ONE_PAIR             = "ワンペア"
	HIGH_CARD            = "ハイカード"

	SPADE   = "s"
	HEART   = "h"
	DIAMOND = "d"
	CLUB    = "c"
)

func main() {
	e := echo.New()
	e.POST("/results", hdl)
	e.Logger.Fatal(e.Start(":8080"))
}

func hdl(c echo.Context) error {

	req := new(Request)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": INVALID_FORMAT})
	}

	var errHand Error
	var errHands []Error
	var strongestPoint int
	var indexStrongestHands []int
	var strongestRank []int
	var correctHands []Hand

	// 役の判定
	for i := 0; i < len(req.Hands); i++ {
		// IDと手札の割り振り
		hand := Hand{
			RequestId: fmt.Sprintf("01-00002-%02d", i+1),
			Hand:      req.Hands[i],
		}

		// 手札をカード配列に分割
		cards := strings.Split(hand.Hand, ", ")

		// スーツとランクの受け取り
		hand.Cards = make([]Card, len(cards))
		for j, card := range cards {
			if card != "" {
				hand.Cards[j].Suit = string(card[0])
				hand.Cards[j].Rank, _ = strconv.Atoi(card[1:])
			}
		}

		// 役判定
		evaluatedHand, err := evaluateHand(hand.Cards)
		if err != nil {
			errHand = Error{
				RequestId:    hand.RequestId,
				Hand:         hand.Hand,
				ErrorMessage: err.Error(),
			}
			errHands = append(errHands, errHand)
			continue
		}

		hand.EvaluatedHand = evaluatedHand
		hand.Point = givePoint(hand.EvaluatedHand)

		if err = hand.Insert(); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"message": INTERNAL_SERVER_ERROR})
		}

		// 最も強い役のインデックスを収集
		if hand.Point == strongestPoint {
			indexStrongestHands = append(indexStrongestHands, len(correctHands))
			strongestRank = append(strongestRank, getStrongestRank(hand.Cards, hand.Point))
		} else if strongestPoint < hand.Point {
			strongestPoint = hand.Point
			indexStrongestHands = []int{len(correctHands)}
			strongestRank = []int{getStrongestRank(hand.Cards, hand.Point)}
		}

		correctHands = append(correctHands, hand)
	}

	// 強さ判定
	for i := 0; i < len(indexStrongestHands); i++ {
		handIndex := indexStrongestHands[i]
		if strongestRank[i] == slices.Max(strongestRank) {
			correctHands[handIndex].Strongest = true
		} else {
			correctHands[handIndex].Strongest = false
		}
	}

	return c.JSON(http.StatusOK, Response{
		Results: correctHands,
		Errors:  errHands,
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
	copySuits := make([]string, len(suits))
	copy(copySuits, suits)
	uniqueSuits := slices.Compact(copySuits)

	return len(uniqueSuits) == 1
}

func makeUniqueRanks(ranks []int) []int {
	copyRanks := make([]int, len(ranks))
	copy(copyRanks, ranks)
	uniqueRanks := slices.Compact(copyRanks)
	return uniqueRanks
}

func groupRanks(ranks []int) [][]int {
	uniqueRanks := makeUniqueRanks(ranks)
	groupedRanks := make([][]int, len(makeUniqueRanks(ranks)))

	for i := 0; i < len(uniqueRanks); i++ {
		for j := 0; j < len(ranks); j++ {
			if uniqueRanks[i] == ranks[j] {
				groupedRanks[i] = append(groupedRanks[i], uniqueRanks[i])
			}
		}
	}
	return groupedRanks
}

func evaluateHand(cards []Card) (string, error) {

	if len(cards) != 5 {
		return "", errors.New(INVALID_HAND_LENGTH)
	}
	if checkDuplication(cards) {
		return "", errors.New(INVALID_SAME_CARDS)
	}

	suits := getSuits(cards)
	ranks := getRanks(cards)

	for i := 0; i < len(cards); i++ {
		if suits[i] == "" {
			return "", errors.New(INVALID_HAND_LENGTH)
		}
		if !(suits[i] == SPADE || suits[i] == CLUB || suits[i] == DIAMOND || suits[i] == HEART) {
			return "", errors.New(INVALID_CARD)
		}
		if !(1 <= ranks[i] && ranks[i] <= 13) {
			return "", errors.New(INVALID_CARD)
		}
	}

	uniqueRanks := makeUniqueRanks(ranks)
	groupedRanks := groupRanks(ranks)

	switch len(uniqueRanks) {
	case 5:
		if isRoyalStraightFlush(suits, ranks) {
			return ROYAL_STRAIGHT_FLUSH, nil
		} else if isStraightFlush(suits, ranks) {
			return STRAIGHT_FLUSH, nil
		} else if isSingleSuits(suits) {
			return FLUSH, nil
		} else if isStraight(ranks) || isRoyalStraight(ranks) {
			return STRAIGHT, nil
		} else {
			return HIGH_CARD, nil
		}
	case 4:
		return ONE_PAIR, nil
	case 3:
		if len(groupedRanks[0]) == 3 || len(groupedRanks[1]) == 3 || len(groupedRanks[2]) == 3 {
			return THREE_OF_A_KIND, nil
		} else if len(groupedRanks[0]) == 2 || len(groupedRanks[1]) == 2 || len(groupedRanks[2]) == 2 {
			return TWO_PAIR, nil
		}
	case 2:
		if len(groupedRanks[0]) == 4 || len(groupedRanks[1]) == 4 {
			return FOUR_OF_A_KIND, nil
		} else if len(groupedRanks[0]) == 3 || len(groupedRanks[1]) == 3 {
			return FULL_HOUSE, nil
		}
	case 1:
		return "", errors.New(INVALID_SAME_RANK)
	}

	return HIGH_CARD, nil
}

func isRoyalStraightFlush(suits []string, ranks []int) bool {
	isFlush := isSingleSuits(suits)
	isRoyalStraight := isRoyalStraight(ranks)

	if isFlush && isRoyalStraight {
		return true
	}
	return false
}

func isStraightFlush(suits []string, ranks []int) bool {
	isFlush := isSingleSuits(suits)
	isStraight := isStraight(ranks)

	if isFlush && isStraight {
		return true
	}
	return false
}

func isStraight(ranks []int) bool {
	uniqueRanks := makeUniqueRanks(ranks)
	isStraight := false

	if slices.Max(uniqueRanks)-slices.Min(uniqueRanks) == 4 {
		isStraight = true
	}
	return isStraight
}

func isRoyalStraight(ranks []int) bool {
	uniqueRanks := makeUniqueRanks(ranks)
	royalStraight := []int{1, 10, 11, 12, 13}
	isRoyalStraight := false

	if slices.Equal(uniqueRanks, royalStraight) {
		isRoyalStraight = true
	}
	return isRoyalStraight
}

func givePoint(evaluatedHand string) int {
	switch evaluatedHand {
	case ROYAL_STRAIGHT_FLUSH:
		return 10
	case STRAIGHT_FLUSH:
		return 9
	case FOUR_OF_A_KIND:
		return 8
	case FULL_HOUSE:
		return 7
	case FLUSH:
		return 6
	case STRAIGHT:
		return 5
	case THREE_OF_A_KIND:
		return 4
	case TWO_PAIR:
		return 3
	case ONE_PAIR:
		return 2
	case HIGH_CARD:
		return 1
	}
	return 1
}

func getStrongestRank(cards []Card, strongestPoint int) int {
	var strongestRank int
	ranks := getRanks(cards)
	groupedRanks := groupRanks(ranks)

	switch strongestPoint {

	case 2:
		for i := 0; i < len(groupedRanks); i++ {
			if len(groupedRanks[i]) == 2 {
				if groupedRanks[i][0] == 1 {
					strongestRank = 14
				} else if strongestRank <= groupedRanks[i][0] {
					strongestRank = groupedRanks[i][0]
				}
			}
		}

	case 3:
		for i := 0; i < len(groupedRanks); i++ {
			if len(groupedRanks[i]) == 2 {
				if groupedRanks[i][0] == 1 {
					strongestRank = 14
				} else if strongestRank <= groupedRanks[i][0] {
					strongestRank = groupedRanks[i][0]
				}
			}
		}

	case 4:
		for i := 0; i < len(groupedRanks); i++ {
			if len(groupedRanks[i]) == 3 {
				if groupedRanks[i][0] == 1 {
					strongestRank = 14
				} else if strongestRank <= groupedRanks[i][0] {
					strongestRank = groupedRanks[i][0]
				}
			}
		}

	case 5:
		for i := 0; i < len(ranks); i++ {
			if isRoyalStraight(ranks) {
				strongestRank = 14
			} else if strongestRank <= ranks[i] {
				strongestRank = ranks[i]
			}
		}

	case 6:
		for i := 0; i < len(ranks); i++ {
			if ranks[i] == 1 {
				strongestRank = 14
			} else if strongestRank <= ranks[i] {
				strongestRank = ranks[i]
			}
		}

	case 7:
		for i := 0; i < len(groupedRanks); i++ {
			if len(groupedRanks[i]) == 3 {
				if groupedRanks[i][0] == 1 {
					strongestRank = 14
				} else if strongestRank <= groupedRanks[i][0] {
					strongestRank = groupedRanks[i][0]
				}
			}
		}

	case 8:
		for i := 0; i < len(groupedRanks); i++ {
			if len(groupedRanks[i]) == 4 {
				if groupedRanks[i][0] == 1 {
					strongestRank = 14
				} else if strongestRank <= groupedRanks[i][0] {
					strongestRank = groupedRanks[i][0]
				}
			}
		}

	case 9:
		for i := 0; i < len(ranks); i++ {
			if isRoyalStraight(ranks) {
				strongestRank = 14
			} else if strongestRank <= ranks[i] {
				strongestRank = ranks[i]
			}
		}

	case 1:
		for i := 0; i < len(ranks); i++ {
			if ranks[i] == 1 {
				strongestRank = 14
			} else if strongestRank <= ranks[i] {
				strongestRank = ranks[i]
			}
		}
	}

	return strongestRank
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

func (hand *Hand) Insert() (err error) {
	statement := `
	INSERT INTO poker_results (request_id, hand, result, timestamp)
	VALUES ($1, $2, $3, now())`

	fmt.Printf("RequestId: %s, Hand: %s, EvaluatedHand: %s\n", hand.RequestId, hand.Hand, hand.EvaluatedHand)

	stmt, err := Db.Prepare(statement)
	if err != nil {
		return err
	}

	defer stmt.Close()

	result, err := stmt.Exec(hand.RequestId, hand.Hand, hand.EvaluatedHand)
	if err != nil {
		log.Printf("Failed to execute insert statement: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Failed to fetch affected rows: %v", err)
		return err
	}

	fmt.Printf("Rows affected: %d\n", rowsAffected)
	return nil
}

func CreateTabe(Db *sql.DB) error {
	// テーブル作成のクエリ
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS poker_results (
		id SERIAL PRIMARY KEY,
		request_id VARCHAR(255),
		hand VARCHAR(255),
		result VARCHAR(255),
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`

	// テーブル作成実行
	_, err := Db.Exec(createTableQuery)
	if err != nil {
		fmt.Println(TABLE_CREATION_FAILURE)
		return err
	} else {
		fmt.Println(TABLE_CREATION_SUCCESSFUL)
		return nil
	}
}
