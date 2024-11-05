# Stockic Newsfeed API System

## Introduction to Stockic Newsfeed System
This is the Stockic API Server which provides API for the Android and IOS app. It has two components which combine to create a fully functional newsfeed API curated with AI:

1. News Feed Endpoint API—The interface with which the App and the Admin communicate for registration, deregistration, and newsfeed. 
2. News Feed Curator - Gathers news from various trusted news agencies through API and curates it with AI.

The News Feed Endpoint API, namely API, is exposed to the internet and is accessible by admin and users for specific purposes. The News Feed Curator, namely the feed-curator, collects news and curates it via AI prompt APIs. 

The feed-curator stores data in a Redis database, which is accessible by API to the server's users. The feed-curator collects and summarizes the data in batch routines as configured and stores it in Redis. Both the API and the feed-curator are independent of each other. 

## API Endpoints (under development)

⚠️ Note: The API is still under development and might change in terms of endpoint URLs and request-response format. Make sure to update your app according to the changes to prevent breaking it. Once the first release is developed, stable releases will be made. 

### Versioning 

Since the API will continue to develop, we will provide various releases, including stable and nightly releases, whose version numbers will be updated here. 

Stable Release (none): Production-ready releases that are tested and verified by the developers. 
Nightly Release (v1): Under development releases for experimental use, which are changing and might contain bugs (not for production use!).

The structure of API is: `/api/<version>/<operation>/<optional-parameters>`

The version would be like: v1, v2, etc., and will follow up in the future.

### API Endpoints

The following are the api endpoints:

#### Registration API 

Endpoint: `/api/<version>/register`
Method: POST
Privilege :Admin
