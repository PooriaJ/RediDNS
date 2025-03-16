# DNS Server

A high-performance, feature-rich DNS server with a RESTful API for DNS zone and record management. This project provides a complete solution for hosting authoritative DNS services with built-in caching and database persistence.

## Features

- **Authoritative DNS Server**: Serves DNS records for your domains
- **RESTful API**: Manage DNS zones and records via a simple HTTP API
- **Multiple Record Types**: Supports A, AAAA, CNAME, MX, NS, PTR, SOA, SRV, TXT, and CAA records
- **Caching**: Redis-based caching for improved performance
- **Persistence**: MariaDB storage for DNS zones and records
- **Real-time Updates**: Instant DNS record updates via Redis pub/sub
- **Docker Support**: Easy deployment with Docker and Docker Compose
- **Configurable**: Flexible configuration options

## Architecture

The DNS Server consists of the following components:

- **DNS Server**: Handles DNS queries using the `miekg/dns` library
- **API Server**: Provides a RESTful API for managing DNS zones and records
- **Redis**: Used for caching DNS records and pub/sub for real-time updates
- **MariaDB**: Stores DNS zones and records

## Prerequisites

- Go 1.21 or higher (for building from source)
- Docker and Docker Compose (for containerized deployment)
- Redis (for caching and pub/sub)
- MariaDB (for data persistence)

## Installation

### Using Docker Compose (Recommended)

1. Clone the repository:
   ```bash
   git clone https://github.com/PooriaJ/RediDNS.git
   cd dns-server
   ```

2. Configure the application by editing `config/config.yaml` if needed

3. Start the services using Docker Compose:
   ```bash
   docker-compose up -d
   ```

4. Verify that the services are running:
   ```bash
   docker-compose ps
   ```

### Building from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/PooriaJ/RediDNS.git
   cd dns-server
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build -o dns-server ./cmd/main.go
   ```

4. Configure the application by editing `config/config.yaml`

5. Run the application:
   ```bash
   ./dns-server
   ```

## Configuration

The application is configured using the `config/config.yaml` file. Here's an example configuration:

```yaml
dns:
  port: 53
  address: 0.0.0.0
  protocol: udp
  soa:
    primary_nameserver: ns1.example.com    # The primary authoritative nameserver for the zone
    mail_address: hostmaster.example.com  # The email address of the administrator responsible for the zone
    refresh: 86400                        # Time (in seconds) before secondary servers refresh their zone data
    retry: 7200                           # Time (in seconds) before a failed zone transfer is retried
    expire: 604800                        # Time (in seconds) before a zone is considered expired if it cannot be refreshed
    minimum: 180                           # Minimum TTL for negative caching, specifying how long non-existent records are cached

api:
  port: 8080
  address: 0.0.0.0

redis:
  address: redis:6379
  password: ""
  db: 0
  cache:
    ttl: 0  # Cache TTL in seconds, 0 means cache forever (until explicit purge)

mariadb:
  host: mariadb
  port: 3306
  user: root
  password: 123
  dbname: dns_server

log:
  level: info
  file: ""
```

### Configuration Options

#### DNS Server
- `dns.port`: The port on which the DNS server listens (default: 53)
- `dns.address`: The address on which the DNS server listens (default: 0.0.0.0)
- `dns.protocol`: The protocol used by the DNS server (default: udp)
- `dns.soa.primary_nameserver`: The authoritative nameserver (default: `ns1.example.com`)
- `dns.soa.mail_address`: The email address of the DNS administrator (default: `hostmaster@example.com`)
- `dns.soa.refresh`: The refresh time for secondary servers (default: 86400)
- `dns.soa.retry`: The retry time for failed zone transfers (default: 7200)
- `dns.soa.expire`: The expire time for zones (default: 604800)
- `dns.soa.minimum`: Minimum TTL for negative caching (default: 180)

#### API Server
- `api.port`: The port on which the API server listens (default: 8080)
- `api.address`: The address on which the API server listens (default: 0.0.0.0)

#### Redis
- `redis.address`: The address of the Redis server (default: localhost:6379)
- `redis.password`: The password for the Redis server (default: "")
- `redis.db`: The Redis database to use (default: 0)
- `redis.cache.ttl`: The TTL for cached records in seconds, 0 means cache forever (default: 0)

#### MariaDB
- `mariadb.host`: The hostname of the MariaDB server (default: localhost)
- `mariadb.port`: The port of the MariaDB server (default: 3306)
- `mariadb.user`: The username for the MariaDB server (default: root)
- `mariadb.password`: The password for the MariaDB server (default: 123)
- `mariadb.dbname`: The name of the MariaDB database (default: dns_server)

#### Logging
- `log.level`: The log level (default: info)
- `log.file`: The log file path, empty means log to stdout (default: "")

## API Documentation

The DNS Server provides a RESTful API for managing DNS zones and records. The API is documented using Swagger and is available at `/api/v1/swagger.json`.

### API Endpoints

#### Health Check
- `GET /api/v1/health`: Check the health of the API server

#### Zones
- `GET /api/v1/zones`: List all zones
- `POST /api/v1/zones`: Create a new zone
- `GET /api/v1/zones/{name}`: Get a zone by name
- `DELETE /api/v1/zones/{name}`: Delete a zone

#### Records
- `GET /api/v1/zones/{zone}/records`: List all records in a zone
- `POST /api/v1/zones/{zone}/records`: Create a new record in a zone
- `GET /api/v1/zones/{zone}/records/{id}`: Get a record by ID
- `PUT /api/v1/zones/{zone}/records/{id}`: Update a record
- `DELETE /api/v1/zones/{zone}/records/{id}`: Delete a record

#### Statistics
- `GET /api/v1/stats`: Get DNS server statistics

## Usage Examples

### Creating a Zone

```bash
curl -X POST http://localhost:8080/api/v1/zones \
  -H "Content-Type: application/json" \
  -d '{"name":"example.com"}'  
```

### Creating an A Record

```bash
curl -X POST http://localhost:8080/api/v1/zones/example.com/records \
  -H "Content-Type: application/json" \
  -d '{
    "name": "www",
    "type": "A",
    "content": "192.168.1.1",
    "ttl": 3600
  }'
```

### Listing Records in a Zone

```bash
curl -X GET http://localhost:8080/api/v1/zones/example.com/records
```

### Updating a Record

```bash
curl -X PUT http://localhost:8080/api/v1/zones/example.com/records/1 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "www",
    "type": "A",
    "content": "192.168.1.2",
    "ttl": 3600
  }'
```

### Deleting a Record

```bash
curl -X DELETE http://localhost:8080/api/v1/zones/example.com/records/1
```

## Testing DNS Resolution

Once you have added some records, you can test DNS resolution using tools like `dig` or `nslookup`:

```bash
dig @localhost www.example.com
```

or

```bash
nslookup www.example.com localhost
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.