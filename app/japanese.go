// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// [START gae_go111_app]

package main

// [START import]
import (
	//"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	//"strconv"
	"strings"
	"unicode/utf8"

	//"strconv"

	//"strings"

	"log"
	"net/http"
	"os"

	"context"
	"time"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	//"github.com/hedhyw/rex/pkg/rex"  // regex builder

	"github.com/gorilla/mux"

	// Note: If connecting using the App Engine Flex Go runtime, use
	// "github.com/jackc/pgx/stdlib" instead, since v4 requires
	// Go modules which are not supported by App Engine Flex.
	_ "github.com/jackc/pgx/v4/stdlib"
)

// [END import]
// [START main_func]

var client *mongo.Client
var db *mongo.Database
var storiesCollection *mongo.Collection
var jmdictCollection *mongo.Collection
var kanjiCollection *mongo.Collection

var tok *tokenizer.Tokenizer

const SQL_FILE = "../testsql.db"
const USER_ID = 0 // TODO for now we hardcode for just one user

func main() {
	var err error
	tok, err = tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		panic(err)
	}

	makeSqlDB()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	db = client.Database("JapaneseEnglish")
	storiesCollection = db.Collection("stories")
	jmdictCollection = db.Collection("jmdict")
	kanjiCollection = db.Collection("kanjidict")

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = client.Ping(ctx, readpref.Primary())

	//fmt.Print(client, err)

	// [START setting_port]
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	router := mux.NewRouter()

	router.HandleFunc("/read/{id}", ReadEndpoint).Methods("GET")
	router.HandleFunc("/word_search", PostWordSearch).Methods("POST")
	router.HandleFunc("/mark/{action}/{id}", MarkStoryEndpoint).Methods("GET")
	router.HandleFunc("/story", CreateStoryEndpoint).Methods("POST")
	router.HandleFunc("/story/{id}", GetStoryEndpoint).Methods("GET")
	router.HandleFunc("/story_retokenize/{id}", RetokenizeStoryEndpoint).Methods("GET")
	router.HandleFunc("/stories_list", GetStoriesListEndpoint).Methods("GET")
	router.HandleFunc("/kanji", KanjiEndpoint).Methods("POST")
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("../static")))

	log.Printf("Listening on port %s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
	// [END setting_port]
}

func makeSqlDB() {
	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	statement, err := sqldb.Prepare("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := statement.Exec(); err != nil {
		log.Fatal(err)
	}

	statement, err = sqldb.Prepare("CREATE TABLE IF NOT EXISTS known_words (id INTEGER PRIMARY KEY, user INTEGER NOT NULL, word INTEGER NOT NULL, FOREIGN KEY(user) REFERENCES users(id))")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := statement.Exec(); err != nil {
		log.Fatal(err)
	}

	statement, err = sqldb.Prepare("CREATE TABLE IF NOT EXISTS stories (id INTEGER PRIMARY KEY, user INTEGER NOT NULL, story TEXT NOT NULL, state TEXT NOT NULL, FOREIGN KEY(user) REFERENCES users(id))")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := statement.Exec(); err != nil {
		log.Fatal(err)
	}

	// statement, err = sqldb.Prepare("INSERT INTO users (name) VALUES (?)")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// statement.Exec("Mario Mario")
	// rows, _ := sqldb.Query("SELECT id, name FROM users")
	// var id int
	// var name string
	// for rows.Next() {
	// 	rows.Scan(&id, &name)
	// 	fmt.Println(strconv.Itoa(id) + ": " + name)
	// }
}

// [END main_func]

// [START indexHandler]

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := storiesCollection.InsertOne(ctx, story)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	json.NewEncoder(response).Encode(result)
}

func GetStoriesListEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	var stories []Story
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := options.Find().SetProjection(bson.D{{"title", 1}, {"_id", 1}})
	cursor, err := storiesCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var story Story
		cursor.Decode(&story)
		stories = append(stories, story)
	}
	if err := cursor.Err(); err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}

	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer sqldb.Close()

	rows, err := sqldb.Query(`SELECT story, state FROM stories WHERE user = $1;`, USER_ID)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to get story: " + err.Error() + `"}`))
		return
	}
	defer rows.Close()

	states := make(map[string]string)
	for rows.Next() {
		var state string
		var storyId string
		if err := rows.Scan(&storyId, &state); err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + "failure to read story states: " + err.Error() + `"}`))
			return
		}
		states[storyId] = state
		fmt.Println("STATUS", storyId, state)
	}

	activeStories := make([]Story, 0)
	inactiveStories := make([]Story, 0)
	unreadStories := make([]Story, 0)
	for _, story := range stories {
		state, ok := states[story.ID.Hex()]
		if !ok || state == "unread" {
			unreadStories = append(unreadStories, story)
		} else if state == "inactive" {
			inactiveStories = append(inactiveStories, story)
		} else if state == "active" {
			activeStories = append(activeStories, story)
		}
	}

	json.NewEncoder(response).Encode(bson.M{
		"unreadStories":   unreadStories,
		"inactiveStories": inactiveStories,
		"activeStories":   activeStories})
}

func ReadEndpoint(response http.ResponseWriter, request *http.Request) {
	fmt.Println(request.URL.Path)
	http.ServeFile(response, request, "../static/index.html")
}

func GetStoryEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	params := mux.Vars(request)
	id, _ := primitive.ObjectIDFromHex(params["id"])
	fmt.Println("story id: ", id)
	var story Story
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := storiesCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&story)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}

	tokenDefinitions := make([][]JMDictEntry, len(story.Tokens))

	for i, token := range story.Tokens {
		tokenDefinitions[i] = make([]JMDictEntry, len(token.Definitions))
		for j, def := range token.Definitions {
			var entry JMDictEntry
			err := jmdictCollection.FindOne(ctx, bson.M{"_id": def}).Decode(&entry)
			if err != nil {
				response.WriteHeader(http.StatusInternalServerError)
				response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
				return
			}
			tokenDefinitions[i][j] = entry
		}
	}

	json.NewEncoder(response).Encode(bson.M{
		"story":       story,
		"definitions": tokenDefinitions,
	})
}

func KanjiEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var str string
	json.NewDecoder(request.Body).Decode(&str)

	var re = regexp.MustCompile(`[\x{4E00}-\x{9FAF}]`)
	kanji := re.FindAllString(str, -1)

	if len(kanji) == 0 {
		json.NewEncoder(response).Encode(bson.M{
			"kanji": bson.A{}})
		return
	}

	arr := bson.A{}
	for _, k := range kanji {
		arr = append(arr, bson.D{{"literal", k}})
	}
	kanjiQuery := bson.D{{Key: "$or", Value: arr}}

	cursor, err := kanjiCollection.Find(ctx, kanjiQuery)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer cursor.Close(ctx)

	kanjiCharacters := make([]KanjiCharacter, 0)

	for cursor.Next(ctx) {
		var ch KanjiCharacter
		cursor.Decode(&ch)
		kanjiCharacters = append(kanjiCharacters, ch)
	}

	json.NewEncoder(response).Encode(bson.M{
		"kanji": kanjiCharacters})
}

func PostWordSearch(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")

	var wordSearch WordSearch
	json.NewDecoder(request.Body).Decode(&wordSearch)

	fmt.Printf("\nword search: %v\n", wordSearch.Word)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var field string

	var re = regexp.MustCompile(`[\x{4E00}-\x{9FAF}]`)
	kanji := re.FindAllString(wordSearch.Word, -1)
	hasKanji := len(re.FindStringIndex(wordSearch.Word)) > 0
	if hasKanji { // if has kanji
		field = "kanji_spellings.kanji_spelling"
	} else {
		field = "readings.reading"
	}

	// only matches at start of string
	startOnlyQuery := bson.D{{field, bson.D{{"$regex", "^" + wordSearch.Word}}}}

	// only matches NOT at start of string
	notStartQuery := bson.D{
		{"$and",
			bson.A{
				bson.D{{field, bson.D{{"$not", bson.D{{"$regex", "^" + wordSearch.Word}}}}}},
				bson.D{{field, bson.D{{"$regex", wordSearch.Word}}}},
			},
		},
	}

	arr := bson.A{}
	for _, k := range kanji {
		arr = append(arr, bson.D{{"literal", k}})
	}
	kanjiQuery := bson.D{{Key: "$or", Value: arr}}

	cursor, err := jmdictCollection.Find(ctx, startOnlyQuery)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer cursor.Close(ctx)
	entriesStart := make([]JMDictEntry, 0)
	for cursor.Next(ctx) {
		var entry JMDictEntry
		cursor.Decode(&entry)
		entriesStart = append(entriesStart, entry)
	}

	cursor, err = jmdictCollection.Find(ctx, notStartQuery)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer cursor.Close(ctx)
	entriesMid := make([]JMDictEntry, 0)
	for cursor.Next(ctx) {
		var entry JMDictEntry
		cursor.Decode(&entry)
		entriesMid = append(entriesMid, entry)
	}

	kanjiCharacters := make([]KanjiCharacter, 0)
	if hasKanji {
		cursor, err = kanjiCollection.Find(ctx, kanjiQuery)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
			return
		}
		defer cursor.Close(ctx)

		for cursor.Next(ctx) {
			var ch KanjiCharacter
			cursor.Decode(&ch)
			kanjiCharacters = append(kanjiCharacters, ch)
		}
	}

	sortResults(entriesStart, hasKanji, wordSearch.Word)
	sortResults(entriesMid, hasKanji, wordSearch.Word)

	nEntriesStart := len(entriesStart)
	if len(entriesStart) > 50 {
		entriesStart = entriesStart[:50]
	}

	nEntriesMid := len(entriesMid)
	if len(entriesMid) > 50 {
		entriesMid = entriesMid[:50]
	}

	json.NewEncoder(response).Encode(bson.M{
		"entries_start": entriesStart,
		"count_start":   nEntriesStart,
		"entries_mid":   entriesMid,
		"count_mid":     nEntriesMid,
		"kanji":         kanjiCharacters})
}

func sortResults(entries []JMDictEntry, hasKanji bool, word string) {
	// compute shortest readings and kanji spellings
	// TODO this could be stored in the DB
	for i := range entries {
		entries[i].ShortestKanjiSpelling = math.MaxInt32
		entries[i].ShortestReading = math.MaxInt32
		for _, ele := range entries[i].K_ele {
			if strings.Contains(ele.Keb, word) {
				count := utf8.RuneCountInString(ele.Keb)
				if count < entries[i].ShortestKanjiSpelling {
					entries[i].ShortestKanjiSpelling = count
				}
			}
		}
		for _, ele := range entries[i].R_ele {
			if strings.Contains(ele.Reb, word) {
				count := utf8.RuneCountInString(ele.Reb)
				if count < entries[i].ShortestReading {
					entries[i].ShortestReading = count
				}
			}
		}
	}

	if hasKanji {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ShortestKanjiSpelling < entries[j].ShortestKanjiSpelling
		})
	} else {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].ShortestReading < entries[j].ShortestReading
		})
	}
}

func MarkStoryEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	params := mux.Vars(request)
	id, err := primitive.ObjectIDFromHex(params["id"])
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}

	// make sure the story actually exists
	var story Story
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	err = storiesCollection.FindOne(ctx, Story{ID: id}).Decode(&story)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}

	action := params["action"]

	if action != "inactive" && action != "unread" && action != "active" {
		response.WriteHeader(400)
		return
	}

	sqldb, err := sql.Open("sqlite3", SQL_FILE)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}
	defer sqldb.Close()

	storyID := story.ID.Hex()

	rows, err := sqldb.Query(`SELECT id FROM stories WHERE story = $1 AND user = $2;`, storyID, USER_ID)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + "failure to get story: " + err.Error() + `"}`))
		return
	}
	exists := rows.Next()
	rows.Close()

	fmt.Println("query ", exists, storyID, USER_ID)

	if exists {
		_, err = sqldb.Exec(`UPDATE stories SET state = $1 WHERE story = $2 AND user = $3;`, action, storyID, USER_ID)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + "failure to update story state: " + err.Error() + `"}`))
			return
		}
	} else {
		_, err = sqldb.Exec(`INSERT INTO stories (story, state, user) VALUES($1, $2, $3);`, storyID, action, USER_ID)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + "failure to insert story state: " + err.Error() + `"}`))
			return
		}
	}

	json.NewEncoder(response).Encode(bson.M{"status": "success"})
}

func RetokenizeStoryEndpoint(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("content-type", "application/json")
	params := mux.Vars(request)
	id, err := primitive.ObjectIDFromHex(params["id"])
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}

	var story Story
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	err = storiesCollection.FindOne(ctx, Story{ID: id}).Decode(&story)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}

	tokens := tok.Analyze(story.Content, tokenizer.Normal)
	story.Tokens = make([]JpToken, len(tokens))

	var re = regexp.MustCompile(`[\x{4E00}-\x{9FAF}]`)

	for i, r := range tokens {
		features := r.Features()
		var searchTerm string
		if len(features) < 9 {
			searchTerm = r.Surface
			story.Tokens[i] = JpToken{
				Surface: r.Surface,
				POS:     features[0],
				POS_1:   features[1],
			}

			//fmt.Println(strconv.Itoa(len(features)), features[0], r.Surface, "features: ", strings.Join(features, ","))
		} else {
			searchTerm = features[6] // base form
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
		var wordQuery primitive.D
		if len(re.FindStringIndex(searchTerm)) > 0 { // has kanji
			//kanji := re.FindAllString(searchTerm, -1)
			wordQuery = bson.D{{"kanji_spellings.kanji_spelling", searchTerm}}
		} else {
			wordQuery = bson.D{{"readings.reading", searchTerm}}
		}

		start := time.Now()
		// Code to measure

		cursor, err := jmdictCollection.Find(ctx, wordQuery)
		if err != nil {
			response.WriteHeader(http.StatusInternalServerError)
			response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
			return
		}
		defer cursor.Close(ctx)

		duration := time.Since(start)

		wordIDs := make([]primitive.ObjectID, 0)
		for cursor.Next(ctx) {
			var entry JMDictEntry
			cursor.Decode(&entry)
			wordIDs = append(wordIDs, entry.ID)
		}

		// past certain point, too many matching words isn't useful (will require manual assignment of definition to the token)

		fmt.Printf("\"%v\" \t matches: %v \t %v \n ", searchTerm, len(wordIDs), duration)
		if len(wordIDs) < 8 {
			story.Tokens[i].Definitions = wordIDs
		}
	}

	_, err = storiesCollection.UpdateByID(ctx, id, bson.M{"$set": story})
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(`{ "message": "` + err.Error() + `"}`))
		return
	}

	json.NewEncoder(response).Encode(story)
}

// [END indexHandler]
// [END gae_go111_app]
