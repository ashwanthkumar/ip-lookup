# ip-lookup

Code to download the IpInfo free jsonl dataset and setup a lookup endpoint using sqlite as the backing store.

It supports indexing both ipv4 and ipv6 IPs and provide an endpoint:

```
GET /lookup/<ip_address>
```

Example response:

```
{
  "ip": "35.154.199.208",
  "country_name": "India",
  "continent_name": "Asia"
}
```

## Usage

Build the binary locally using

```
go mod download
go build .
```

Run it via Docker.

```
docker run --restart=always -p 8080:8080 -v $(pwd):/app/data -e IP_DATA_URL="https://ipinfo.io/data/free/country.json.gz?token=your_actual_token" ashwanthkumar/ip-lookup:latest
```

1. Update `your_actual_token` with the actual token from the IP Info Dashboard.
2. Feel free to change the port where the app runs as required.
3. Make sure you mount a local folder into `/app/data` directory.

## Execution

Update the `token` from your dashboard from https://ipinfo.io/account/data-downloads. We download and
host the "Free IP to Country" dataset.

```
IP_DATA_URL="https://ipinfo.io/data/free/country.json.gz?token=..." ./ip-lookup
```

## License
MIT


## Credits
Thanks to Claude for generating most of the code in this repo.
