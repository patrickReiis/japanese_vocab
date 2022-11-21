package main

import (
	"encoding/xml"
	"fmt"
	"regexp"

	"context"
	// "go.mongodb.org/mongo-driver/bson"
	//"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"time"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	//"io/ioutil"
	"os"
	"strconv"
	"strings"
)

var client *mongo.Client
var db *mongo.Database
var wiktionaryCollection *mongo.Collection

var tok *tokenizer.Tokenizer

func main() {
	var err error
	tok, err = tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		panic(err)
	}
	// wakati
	fmt.Println("---wakati---")
	seg := tok.Wakati("すもももももももものうち")
	fmt.Println(seg)

	// tokenize
	fmt.Println("---tokenize---")
	tokens := tok.Analyze("すもももももももものうち", tokenizer.Search)
	for _, token := range tokens {
		features := strings.Join(token.Features(), ",")
		fmt.Printf("%s\t%v\n", token.Surface, features)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err = mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))

	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	db = client.Database("JapaneseEnglish")
	wiktionaryCollection = db.Collection("wiktionary")

	ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = client.Ping(ctx, readpref.Primary())
	//makeCollection()
	words := processCollection()

	for _, w := range words {
		err := createWord(w)
		if err != nil {
			fmt.Println(err)
			break
		}
	}
}

func createWord(word Word) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	_, err := wiktionaryCollection.InsertOne(ctx, word)
	return err
}

func processCollection() []Word {
	// Open our xmlFile
	xmlFile, err := os.Open("../../enwiktionary-collection.xml")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Successfully Opened xml")
	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	words := make([]Word, 0)

	decoder := xml.NewDecoder(xmlFile)

	re, err := regexp.Compile(`^[\w\s]+$`)
	if err != nil {
		panic(err)
	}

	for {
		// Read tokens from the XML document in a stream.
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		// Inspect the type of the token just read.
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "word" {
				var word Word
				decoder.DecodeElement(&word, &se)

				if !re.MatchString(word.Text) {
					words = append(words, word)
					//fmt.Println(word.Text)
				} else {
					//fmt.Println(word.Text)
				}
			}
		default:
		}
	}

	fmt.Println("done decoding. Number of words:", len(words))

	return words
}

func makeCollection() {
	// Open our xmlFile
	xmlFile, err := os.Open("../../enwiktionary-20221101-pages-meta-current.xml")
	//xmlFile, err := os.Open("./text.xml")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Successfully Opened xml")
	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	words := make([]Word, 0)

	decoder := xml.NewDecoder(xmlFile)

	for {
		// Read tokens from the XML document in a stream.
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		// Inspect the type of the token just read.
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == "page" {
				var page Page
				decoder.DecodeElement(&page, &se)
				if page.Namespace == 0 && strings.Contains(page.Revision.Text, "==Japanese==") {
					words = append(words, Word{page.Title, page.Revision.Text})
					fmt.Println("Number of pages:", strconv.Itoa(len(words)), page.Title)
				}
			}
		default:
		}
	}

	outputFile, err := os.Create("../../enwiktionary-collection.xml")
	if err != nil {
		fmt.Println(err)
	}

	defer outputFile.Close()

	encoder := xml.NewEncoder(outputFile)

	encoder.Indent("", "  ")
	if err := encoder.Encode(WordCollection{words}); err != nil {
		panic(err)
	}

	fmt.Println("done encoding")
}

func parseRevisionText(text string) Word {
	fmt.Println(text)
	var word Word
	return word
}

type WordCollection struct {
	Words []Word `xml:"word"`
}

type Word struct {
	Text       string `xml:"text"`
	Definition string `xml:"definition"`
}

type Revision struct {
	/*
	   	<id>1692687</id>
	         <timestamp>2022-08-20T11:55:00Z</timestamp>
	         <contributor>
	           <username>Mtodo</username>
	           <id>329</id>
	         </contributor>
	         <comment>[[权现]]を繁体字化</comment>
	         <model>wikitext</model>
	         <format>text/x-wiki</format>
	         <text
	*/
	Id        int    `xml:"id"`
	Timestamp string `xml:"timestamp"`
	Format    string `xml:"format"`
	Text      string `xml:"text"`
}

type Page struct {
	Title     string   `xml:"title"`
	Namespace int      `xml:"ns"`
	Id        int      `xml:"id"`
	Revision  Revision `xml:"revision"`
}

type MediaWiki struct {
	Siteinfo string `xml:"siteinfo"`
	Pages    []Page `xml:"page"`
}
