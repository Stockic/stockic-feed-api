# Stockic Newsfeed API System

## Finalised Design for the Android and IOS App
<img width="1006" alt="Screenshot 2024-11-06 at 11 59 59" src="https://github.com/user-attachments/assets/0ce4f3ac-85ee-48ea-84d6-c54662566c0c">

## Introduction to Stockic Newsfeed System
This is the Stockic API Server, which provides API for Android and iOS apps. It has two components which combine to create a fully functional newsfeed API curated with AI:

1. News Feed Endpoint API—The interface with which the App and the Admin communicate for registration, deregistration, and newsfeed. 
2. News Feed Curator - Gathers news from various trusted news agencies through API and curates it with AI.

The News Feed Endpoint API, namely API, is exposed to the internet and is accessible to admins and users for specific purposes. The News Feed Curator, namely the feed-curator, collects news and curates it via AI prompt APIs. 

The feed-curator stores data in a Redis database, which is accessible by API to the server's users. The feed-curator collects and summarizes the data in batch routines as configured and stores it in Redis. Both the API and the feed-curator are independent of each other. 

## System Design and Architecture

![image](https://github.com/user-attachments/assets/3c39a65c-83f9-4774-9882-9bf033a8095a)

## API Endpoints (under development)

⚠️ Note: The API is still under development and might change regarding endpoint URLs and request-response format. Ensure you update your app according to the changes to prevent it from breaking. Once the first release is developed, stable releases will be made. 

### Versioning 

Since the API will continue to develop, we will provide various releases, including stable and nightly releases, whose version numbers will be updated here. 

Stable Release (none): Production-ready releases that are tested and verified by the developers. 
Nightly Release (v1): Under development releases for experimental use, which are changing and might contain bugs (not for production use!).

The structure of API is: `/api/<version>/<operation>/<optional-parameters>`

The version would be v1, v2, etc., and will follow up in the future.

### Accessibility Structure 

The API server requires the User to provide the API Key to perform operations. This API key decodes user information like location and tier (free or premium). Every user would have an API key on the app, which would be used to communicate with the api server. Every user is bound to the API Key, and all information would be linked with the API key (technically, X-API-Key). The api server can get info from Firebase and determine the location to serve geolocation-specific data to the caller. 

Hence, to conclude this section, the API key is the primary and only identifier, and each request must contain it. With the API key, the user must only worry about the operations to be performed and leave the rest to the developers of this API. 

### API Endpoints

The following are the API endpoints: 

#### Home Page-Specific APIs
According to the UI provided, the first page contains two portions: headlines (Geolocation-Specific) and Today's Newsfeed (Geolocation-Specific). 

1. Headlines: This API endpoint is geolocation-specific, which would be decoded by the api server itself from Firebase.

Endpoint: `http://api.adityapatil.dev/api/<version>/headlines/<page-size>`
Method: `GET`
Header: `X-API-Key`

This API would serve headlines per the geolocation fetched by the api server. Users would be served with the latest headlines of the given region. 

2. Today's Newsfeed: This API is based on content pagination and is geolocation-specific. The user must send a page number to fetch news.

Endpoint: `http://api.adityapatil.dev/api/<version>/newsfeed/<page-size>/<page-number>`
Method: `GET`
Header: `X-API-Key`

#### Discover Page-Specific APIs
The Discover Page is where users get categorized newsfeeds. Hence, the category must be provided. 

1. Discover: This API endpoint demands category and page number and serves specific content. 

Endpoint: `http://api.adityapatil.dev/api/<version>/discover/<category>/<page-size>/<page-number>`
Method: `GET`
Header: `X-API-Key`

Available categories: `gainers`, `losers`, `software`, `finance`, `stocks`, `bonds`, `corporate`, `banking`, `technology`, `tax`, `geopolitics`

#### Detailed News Page API
As per the UI design, the third picture shows detailed information about the news. This can be fetched with the content ID. 

1. Detailed Newsfeed: Provides detailed information about the news.

Endpoint: `http://api.adityapatil.dev/api/<version>/detail/<news-id>`
Method: `GET`
Header: `X-API-Key`

⚠️ Note: Responses are not mentioned here since they are under design. It would be updated shortly. 
