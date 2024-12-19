# stellr

A full-text search engine, but tiny.

## Usage

Download a release or compile from source with the following command:

```bash
go build .
```

Run the binary file:

```bash
./stellr
```

The RESTful HTTP API will be available on port 8345.

### Uploading a text corpus

The text corpus should be a plain text file with one text document per line. The file should be uploaded to the `uploadCorpus` endpoint. Sample command with curl:

```bash
curl -X POST http://localhost:8345/uploadCorpus -F "corpus=@corpus.txt"
```

You can specify the language, otherwise English is used:

```bash
curl -X POST 'http://localhost:8345/uploadCorpus?language=english' -F "corpus=@corpus.txt"
```

The language is used to remove stop words and, optionally, stemming. To enable stemming, you should pass the `stem` parameter as `true`:

```bash
curl -X POST 'http://localhost:8345/uploadCorpus?language=english&stem=true' -F "corpus=@corpus.txt"
```

The following languages are supported: English, Spanish, French, Russian, Swedish, Norwegian, Hungarian.

### Querying

Sample command with curl:

```bash
curl 'localhost:8345/search?query=memorable'
```

A JSON response such as the following is returned:

```json
[
  {
    "text": "Recycled and predictable plot. The characters are as memorable as the story line. We came in few minutes late and only saw the end of the opening scene which turned out to be a good thing since it was too intense for a 3 and a 4 year old. Overall a disappointment.428_3.txt:",
    "score": 239,
    "id": 6149
  },
  {
    "text": "It's hard to write 10 lines of copy about this so-so film noir. There just isn't a lot to say about it. It is not memorable enough to add to your collection, and I have a considerable amount of noirs.<br /><br />Paul Henreid plays a tough guy in here. He's not one I would think of to play this kind of role, but he's fine with it. He's a fine actor, anyway.<br /><br />Everything, including the cinematography, is okay-but-not memorable. One thing that stood out: the abrupt ending. That was a surprise. It was also a surprise to see this under the heading \"Hollow Triumph.\" I've never seen the film called that. It's always been called \"Scar.\"<br /><br />If you read about a \"tense film noir,\" etc., don't believe it. \"Tense\" is not an accurate adjective for this film.11089_1.txt:",
    "score": 226,
    "id": 1211
  }
]
```

#### Types of search

There are three different search types available: exact, prefix, or fuzzy. If no type is specified, exact search is used.

- Exact search: only exact matches to query words are considered
- Prefix search: consider exact matches and prefixes. For instance, the query _great_ will match both _great_ and _greater_
- Fuzzy search: all matches up to a maximum edit distance are considered. This allows for typos and small variations

Some examples:

```bash
curl 'localhost:8345/search?query=great&type=prefix'
```

```bash
curl 'localhost:8345/search?query=memorable&type=fuzzy&distance=2'
```

### Search operators

By default, results that contain any of the provided words are returned. That is, an _or_ operator is used. It is possible to use an _and_ operator. With this option, only documents with **all** provided words are returned.

```bash
curl 'localhost:8345/search?query=memorable%20great&operator=and'
```
