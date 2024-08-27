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
