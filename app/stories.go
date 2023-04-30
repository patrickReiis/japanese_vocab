package main

// [START import]
import (
	//"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	// "math"
	"regexp"
	// "sort"

	// "strings"
	// "unicode/utf8"

	// //"strconv"

	// "log"
	"net/http"
	// "os"

	"context"
	"time"

	// "github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	// "go.mongodb.org/mongo-driver/mongo"
	//"go.mongodb.org/mongo-driver/mongo/options"
	// "go.mongodb.org/mongo-driver/mongo/readpref"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	//"github.com/hedhyw/rex/pkg/rex"  // regex builder

	"github.com/gorilla/mux"
)

func CreateStoryEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	var story Story
	json.NewDecoder(request.Body).Decode(&story)

	tokens := tok.Analyze(story.Content, tokenizer.Normal)
	story.Tokens = make([]JpToken, len(tokens))

	for i, r := range tokens {
		features := r.Features()
		if len(features) < 9 {

			story.Tokens[i] = JpToken{
				Surface: r.Surface,
				POS:     features[0],
				POS_1:   features[1],
			}

			//fmt.Println(strconv.Itoa(len(features)), features[0], r.Surface, "features: ", strings.Join(features, ","))
		} else {
			story.Tokens[i] = JpToken{
				Surface:          r.Surface,
				POS:              features[0],
				POS_1:            features[1],
				POS_2:            features[2],
				POS_3:            features[3],
				InflectionalType: features[4],
				InflectionalForm: features[5],
				BaseForm:         features[6],
				Reading:          features[7],
				Pronunciation:    features[8],
			}
		}
	}

	err := getDefinitions(story.Tokens, response)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": failure to get definitions"` + err.Error() + `"}`))
		return
	}

	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer sqldb.Close()

	wordIds, err := addDrillWords(story.Tokens, response)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to add words: " + err.Error() + `"}`))
		return
	}

	wordsJson, err := json.Marshal(wordIds)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to marshall wordIds: " + err.Error() + `"}`))
		return
	}

	tokensJson, err := json.Marshal(story.Tokens)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to marshall tokens: " + err.Error() + `"}`))
		return
	}

	_, err = sqldb.Exec(`INSERT INTO stories (user, state, words, content, title, link, tokens) VALUES($1, $2, $3, $4, $5, $6, $7);`,
		USER_ID, "unread", wordsJson, story.Content, story.Title, story.Link, tokensJson)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to insert story state: " + err.Error() + `"}`))
		return
	}
	json.NewEncoder(response).Encode("Success adding story")
}

func addDrillWords(tokens []JpToken, response http.ResponseWriter) ([]int64, error) {
	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return nil, err
	}
	defer sqldb.Close()

	var reHasKanji = regexp.MustCompile(`[\x{4E00}-\x{9FAF}]`)
	var reHasKana = regexp.MustCompile(`[あ-んア-ン]`)
	var reHasKatakana = regexp.MustCompile(`[ア-ン]`)

	// deduplicate
	tokenSet := make(map[string]JpToken)
	for _, token := range tokens {
		tokenSet[token.BaseForm] = token
	}

	tokens = nil
	for k := range tokenSet {
		tokens = append(tokens, tokenSet[k])
	}

	wordIds := make([]int64, 0)
	for _, token := range tokens {
		hasKanji := len(reHasKanji.FindStringIndex(token.BaseForm)) > 0
		hasKana := len(reHasKana.FindStringIndex(token.BaseForm)) > 0
		if !hasKanji && !hasKana {
			continue
		}

		rows, err := sqldb.Query(`SELECT id FROM words WHERE base_form = $1 AND user = $2;`, token.BaseForm, USER_ID)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + "error while looking up word: " + err.Error() + `"}`))
			return nil, err
		}
		exists := rows.Next()

		unixtime := time.Now().Unix()

		var id int64
		if exists {
			rows.Scan(&id)
			wordIds = append(wordIds, id)
			fmt.Printf("getting word: %s %d \t %d\n", token.BaseForm, len(token.Entries), id)
		} else {
			drillType := 0
			hasKatakana := len(reHasKatakana.FindStringIndex(token.BaseForm)) > 0
			if hasKatakana {
				drillType |= DRILL_TYPE_KATAKANA
			}

			for _, entry := range token.Entries {
				for _, sense := range entry.Sense {
					drillType |= getVerbDrillType(sense)
				}
			}

			entriesJson, err := json.Marshal(token.Entries)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + "failure to json encode entry: " + err.Error() + `"}`))
				rows.Close()
				return nil, err
			}

			fmt.Printf("\nadding word: %s %d \t %d\n", token.BaseForm, len(token.Entries), id)

			insertResult, err := sqldb.Exec(`INSERT INTO words (base_form, user, countdown, drill_count, 
					read_count, date_last_read, date_last_drill, date_added, date_last_wrong, definitions, drill_type) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);`,
				token.BaseForm, USER_ID, INITIAL_COUNTDOWN, 0, 0, unixtime, 0, unixtime, 0, entriesJson, drillType)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + "failure to insert word: " + err.Error() + `"}`))
				rows.Close()
				return nil, err
			}

			id, err := insertResult.LastInsertId()
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + "failure to get id of inserted word: " + err.Error() + `"}`))
				rows.Close()
				return nil, err
			}

			wordIds = append(wordIds, id)

		}
		rows.Close()
	}

	return wordIds, nil
}

func getDefinitions(tokens []JpToken, response http.ResponseWriter) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var re = regexp.MustCompile(`[\x{4E00}-\x{9FAF}]`)

	for i, token := range tokens {
		searchTerm := token.Surface

		var wordQuery primitive.D
		if len(re.FindStringIndex(searchTerm)) > 0 { // has kanji
			//kanji := re.FindAllString(searchTerm, -1)
			wordQuery = bson.D{{"kanji_spellings.kanji_spelling", searchTerm}}
		} else {
			wordQuery = bson.D{{"readings.reading", searchTerm}}
		}

		//start := time.Now()

		cursor, err := jmdictCollection.Find(ctx, wordQuery)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
			return err
		}
		defer cursor.Close(ctx)

		//duration := time.Since(start)

		entries := make([]JMDictEntry, 0)
		for cursor.Next(ctx) {
			var entry JMDictEntry
			cursor.Decode(&entry)
			entries = append(entries, entry)
		}

		fmt.Printf("\"%v\" \t\t\t matches: %v \n ", searchTerm, len(entries))

		// past certain point, too many matching words isn't useful (will require manual assignment of definition to the token)
		if len(entries) > 8 {
			entries = entries[:8]
		}

		tokens[i].Entries = entries
	}

	return nil
}

func GetStoriesListEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")

	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer sqldb.Close()

	rows, err := sqldb.Query(`SELECT id, state, words, title, link FROM stories WHERE user = $1;`, USER_ID)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to get story: " + err.Error() + `"}`))
		return
	}
	defer rows.Close()

	var stories []StorySql
	for rows.Next() {
		var story StorySql
		if err := rows.Scan(&story.ID, &story.State, &story.Words, &story.Title, &story.Link); err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + "failure to read story states: " + err.Error() + `"}`))
			return
		}
		stories = append(stories, story)
	}

	json.NewEncoder(response).Encode(stories)
}

func ReadEndpoint(response http.ResponseWriter, request *http.Request) {
	fmt.Println(request.URL.Path)
	http.ServeFile(response, request, "../static/index.html")
}

func GetStoryEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	params := mux.Vars(request)
	id, err := strconv.Atoi(params["id"])
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	fmt.Println("GET STORY id: ", id)

	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer sqldb.Close()

	rows, err := sqldb.Query(`SELECT state, words, title, link, tokens, content FROM stories WHERE user = $1 AND id = $2;`, USER_ID, id)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to get story: " + err.Error() + `"}`))
		return
	}
	defer rows.Close()

	var story StorySql
	for rows.Next() {

		if err := rows.Scan(&story.State, &story.Words, &story.Title, &story.Link, &story.Tokens, &story.Content); err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + "failure to read story states: " + err.Error() + `"}`))
			return
		}
	}

	json.NewEncoder(response).Encode(story)
}

func MarkStoryEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	params := mux.Vars(request)
	storyId, err := strconv.Atoi(params["id"])
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	fmt.Println("MARK STORY id: ", storyId)

	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer sqldb.Close()

	// make sure the story actually exists
	rows, err := sqldb.Query(`SELECT state FROM stories WHERE user = $1 AND id = $2;`, USER_ID, storyId)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to get story: " + err.Error() + `"}`))
		return
	}

	if !rows.Next() {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "story with ID does not exist: " + err.Error() + `"}`))
		rows.Close()
		return
	}
	rows.Close()

	action := params["action"]
	if action != "inactive" && action != "unread" && action != "active" {
		response.WriteHeader(400)
		return
	}

	_, err = sqldb.Exec(`UPDATE stories SET state = $1 WHERE id = $2 AND user = $3;`, action, storyId, USER_ID)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to update story state: " + err.Error() + `"}`))
		return
	}

	json.NewEncoder(response).Encode(bson.M{"status": "success"})
}

// [END indexHandler]
// [END gae_go111_app]
