package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Album struct {
	Id     primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Title  string             `json:"album" bson:"album"`
	Artist string             `json:"artist" bson:"artist"`
	Year   int                `json:"year" bson:"year"`
}

type Albums []Album

var albums Albums
var myCollection *mongo.Collection
var ctx context.Context

func main() {
	ctx = context.Background()
	opt := options.Client().ApplyURI("mongodb://root:rootpassword@gomdb:27017")
	client, err := mongo.Connect(ctx, opt)
	if err != nil {
		log.Fatal(err)
	}
	if err = client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal(err)
	}
	myCollection = client.Database("mydb").Collection("albums")
	albums, err = albumInit(ctx, myCollection)
	if err != nil {
		log.Fatal(err)
	}

	router := gin.Default()
	router.GET("/albums", getAlbums)
	router.POST("/albums", postAlbum)
	router.GET("/albums/:title", getAlbumByTitle)
	router.PUT("/albums", updateAlbum)
	router.DELETE("/albums/:title", deleteAlbumByTitle)
	router.Run(":8080")
}

func albumInit(ctx context.Context, coll *mongo.Collection) (Albums, error) {
	var a = []interface{}{
		Album{Title: "Zeit", Artist: "Rammstein", Year: 2022},
		Album{Title: "A Day at the Races", Artist: "Queen", Year: 1975},
		Album{Title: "9. Symphonie", Artist: "Beethoven", Year: 1824},
	}
	if err := coll.Drop(ctx); err != nil {
		log.Printf("db not dropped %v", err)
	}

	_, err := coll.InsertMany(ctx, a)
	if err != nil {
		return nil, err
	}
	cursor, err := coll.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	var all Albums
	if err = cursor.All(ctx, &all); err != nil {
		return nil, err
	}

	return all, nil
}

func getAlbums(c *gin.Context) {
	cursor, err := myCollection.Find(ctx, bson.D{})
	if err != nil {
		return
	}
	var all Albums
	if err = cursor.All(ctx, &all); err != nil {
		return
	}
	c.IndentedJSON(http.StatusOK, all)
}

func postAlbum(c *gin.Context) {
	var another Album
	if err := c.BindJSON(&another); err != nil {
		return
	}
	rs, err := myCollection.InsertOne(ctx, another)
	if err != nil {
		log.Printf("post failed %v", err)
	}
	id, ok := rs.InsertedID.(primitive.ObjectID)
	if !ok {
		log.Printf("id is not ObjectID: %T", id)
	}
	another.Id = id
	albums = append(albums, another)
	c.IndentedJSON(http.StatusCreated, another)
}

func getAlbumByTitle(c *gin.Context) {
	title := c.Param("title")
	filter := bson.M{"album": title}
	single := myCollection.FindOne(ctx, filter)
	var a Album
	if err := single.Decode(&a); err != nil {
		log.Printf("did not find %s in db: %v", title, err)
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album " + title + " not found"})
		return
	}
	c.IndentedJSON(http.StatusOK, a)
}

func updateAlbum(c *gin.Context) {
	var updated Album
	if err := c.BindJSON(&updated); err != nil {
		return
	}
	upd := bson.D{
		{"$set", bson.M{
			"album":  updated.Title,
			"artist": updated.Artist,
			"year":   updated.Year,
		},
		},
	}

	log.Printf("%v", updated)
	rs, err := myCollection.UpdateByID(ctx, updated.Id, upd)
	if err != nil || rs.UpsertedID != nil {
		log.Printf("could not update %v", err)
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "not updated"})
		return
	}

	var fresh Album
	if err := myCollection.FindOne(ctx, bson.M{"_id": updated.Id}).Decode(&fresh); err != nil {
		log.Printf("did not find updated album %v", err)
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "updated album not found"})
		return
	}
	for i, a := range albums {
		if a.Id == fresh.Id {
			albums[i] = fresh
			break
		}
	}
	c.IndentedJSON(http.StatusOK, fresh)
}

func deleteAlbumByTitle(c *gin.Context) {
	title := c.Param("title")
	filter := bson.M{"album": title}
	_, err := myCollection.DeleteOne(ctx, filter)
	if err != nil {
		log.Printf("could not delete %s: %v", title, err)
		c.IndentedJSON(http.StatusNotFound, gin.H{"message": "album " + title + " not deleted"})
	}
	var delAlbum Album
	for i, a := range albums {
		if a.Title == title {
			n := len(albums) - 1
			delAlbum = a
			albums[i] = albums[len(albums)-1]
			albums = albums[:n]
			break
		}
	}
	c.IndentedJSON(http.StatusOK, delAlbum)
}
