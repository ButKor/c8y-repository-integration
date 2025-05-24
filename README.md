# Attention / Disclaimer

This repository is **work in progress**. It is not ready to use yet. 

# About

A repository to integrate the Cumulocity Firmware Repository with an AWS storage account. The service supports:

* Discovering firmware images stored in an S3 bucket. These images will be synchronized with Cumulocitys Firmware Repositories. It does *not* copy the actual Files to the Cumulocity repository, it works with URLs instead. 

* It exposes an endpoint to download the firmware images. 

See below sequence diagram for details:

```mermaid
sequenceDiagram
    autonumber
    actor User
    box white Cumulocity
    participant CC as Cumulocity<br>Core / UI
    participant FR as Firmware<br>Repository
    participant IS as Integration<br>Microservice
    end
    participant ES as External Storage
    participant D as Device

    Note over User,D: Preparation/Discovery Phase
    User ->> ES: Upload Firmware
    IS ->> ES: Discover all available firmware versions
    ES -->> IS: list image files
    IS ->> FR: Create Firmware versions with e.g. <br>url=https://iot.eu-latest.cumulocity.com/<br>/repo-integration/firmware/download?id=8752912
    Note left of FR: This Firmware Managed Object<br> allows Users to select Firmware in Cumulocity.<br>It has external address of the image persisted.
    FR -->> IS: return Managed Object
    Note over User,D: Schedule Firmware Update
    User ->> CC: Schedule Firmware Update
    CC ->> D: Send Firmware Download instruction<br>with url=https://iot.eu-latest.cumulocity.com/service/repo-integration/firmware/download?id=8752912
    Note over D,IS: Start Download Phase
    D ->> IS: http GET https://iot.eu-latest.cumulocity.com/<br>service/repo-integration/firmware/download?id=8752912
    IS ->> CC: Lookup external storage address from Firmware Managed Object
    CC -->> IS: return Firmware Managed Object
    IS ->> ES: create presigned url via API
    ES -->> IS: reply presigned url
    IS ->> D: instruct device http-client to do<br>url-redirect to presigned-URL
    D ->> ES: Download file
    ES -->> D: Downloaded
    Note over D,IS: End Download Phase
    D -->> D: Apply Firmware Update
    D -->> CC: Log success/failure of firmware update
```

# Configuration

Service can be configured with the below tenant options:

Category | Key | Value
--|--|--|
repo-integration-fw | awsConnectionDetails | '{"region": "{aws region}", "secretAccessKey": "{aws access secret}", "accessKeyID": "{aws access key}", "bucketName": "{bucket name}" }'

# Next steps

* Supporting Azure Blob storage

* Next to firmware, also support software-repository