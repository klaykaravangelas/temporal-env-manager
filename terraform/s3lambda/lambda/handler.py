import json


def handler(event, context):
    for record in event.get("Records", []):
        bucket = record["s3"]["bucket"]["name"]
        key = record["s3"]["object"]["key"]
        size = record["s3"]["object"].get("size", "unknown")
        event_name = record["eventName"]

        print(f"S3 Event: {event_name}")
        print(f"Bucket: {bucket}")
        print(f"Object Key: {key}")
        print(f"Object Size: {size} bytes")

    return {
        "statusCode": 200,
        "body": json.dumps("Event processed successfully")
    }
