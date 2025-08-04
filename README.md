# Implementing Your Own ID HASH Module

## Goal

The goal of this module is very simple. It must match a collection of phone numbers, 
coming from myGaru's partners (**Data Vendors**) to a collection of values at your (**Telecommunications Operator**) discretion, 
referred to here as `telco ident values`.

Each phone must be mapped to one such unique `telco ident value`. One phone number must always correspond to one `telco ident value`.

After this mapping, the ID HASH component forwards the mapped batch to the UID Mapper (UidMap) component for storage and further processing.

This ensures that myGaru can use an anonymized representation of the Data Vendor's phone numbers 
without having access to sensitive data
from either the Data Vendor or the Telecommunications Operator.


## Interface

### Endpoints

You must provide a single HTTP endpoint:

`POST /pim`

**Request parameters:** none.

**Expected responses:**


| Code                      | Description                                                                                                                   | Response Body       |
|---------------------------|-------------------------------------------------------------------------------------------------------------------------------|---------------------|
| 204 No Content            | Return if batch was successfully forwarded to UID Mapper.                                                                     | None.               |
| 500 Internal Server Error | Return if the mapping process failed <br/> OR <br/> UID Mapper returned non-204 code.                                         | Error (plain text). |
| 403 Forbidden             | Return if the X-ClientID header does NOT match the PartnerID in the batch<br/> OR<br/> the request came from unauthorized IP. | None.               |
| 400 Bad Request           | Return if parsing the JSON body failed.                                                                                       | Error (plain text). |
| 405 Method Not Allowed    | Return if request is not POST.                                                                                                | None.               |

**Expected flow:**
1. Validate IP (See [Access to ID HASH](#access-to-id-hash-important)) of the request. If unauthorized, return 403.

2. Parse request body data into expected batch. If this fails, return 400.

Batch structure:
```json
{
  "telco_id": "5cf5ad85-c686-46a0-8f2d-aa0f964b55f6", 
  "partner_id": "74dac49f-12ea-463a-9fe4-1d0e85af7ae3", 
  "pim_id": "013801b7-1f13-45f8-b787-f699691ede55", 
  "data": [
    ["+380585494404", "e60a9b4b-54a6-41d4-97c5-fc900bf7b464"],
    ["+380585494405", "a5b64eba-caa2-4336-9d0b-04a2e2eb9bc5"],
    ["+380585494406", "fa3e5132-30b4-4429-a2fe-3a13c7ca5bcf"],
    ["+380585494407", "0e09c354-9c63-4fca-af77-492a71ed2ab1"]
  ]
}
```

- `telco_id`: represents the UUID of the Telecommunications Operator in the myGaru system.
- `partner_id`: represents the UUID of the Data Vendor which initiated this request.
- `pim_id`: represents the UUID of the request itself.
- `data`: a list of phone numbers along with their tokens. You must simply replace each phone number with the corresponding `telco ident value`.


3. Validate the `X-ClientID` header (this represents the Common Name from the certificate presented via the Auth Middleware).
   **IMPORTANT**: If it does NOT equal `partner_id` from the parsed batch, return 403. 
4. Map the list of phone numbers from the batch to your `telco ident values`, as decribed in the Goal section.
5. Create the body of the request that must be passed along to the UID Mapper component.
   It must have this structure:

```json
{
  "telco_id": "5cf5ad85-c686-46a0-8f2d-aa0f964b55f6", 
  "partner_id": "74dac49f-12ea-463a-9fe4-1d0e85af7ae3", 
  "pim_id": "013801b7-1f13-45f8-b787-f699691ede55", 
  "data": [
    ["11487659064281169587", "e60a9b4b-54a6-41d4-97c5-fc900bf7b464"],
    ["12063760019269155007", "a5b64eba-caa2-4336-9d0b-04a2e2eb9bc5"],
    ["14640892673165708323", "fa3e5132-30b4-4429-a2fe-3a13c7ca5bcf"],
    ["6788698777015378439", "0e09c354-9c63-4fca-af77-492a71ed2ab1"]
  ]
}
```

- `telco_id`, `partner_id`, `pim_id`: simply copy them from the incoming batch.
- `data`: the same list as before, only with `telco ident values` instead of phone numbers. 
- **IMPORTANT**: please ensure that each token is matched with the `telco ident value` corresponding to its previous phone number. 
For example, `"+380585494404"` was matched to `"11487659064281169587"`, therefore its token (`"e60a9b4b-54a6-41d4-97c5-fc900bf7b464"`) must be grouped with `"11487659064281169587"`.


6. Send the formed batch to the UID Mapper component:
- The address of the UID Mapper component and timeout interval should be set in your configuation file. See the Configuration section.
- Set the same `X-ClientID` header as you've received in the incoming request.
- Send the POST request to the UID Mapper address, path `/pim`, with a timeout **NO LARGER THAN 10m**.

Example:
```
POST http://your.uidmap.instance/pim HTTP/1.1
Host: your.uidmap.instance
Content-Type: application/json
Content-Length: 225

{
  "telco_id": "5cf5ad85-c686-46a0-8f2d-aa0f964b55f6", 
  "partner_id": "74dac49f-12ea-463a-9fe4-1d0e85af7ae3", 
  "pim_id": "013801b7-1f13-45f8-b787-f699691ede55", 
  "data": [
    ["11487659064281169587", "e60a9b4b-54a6-41d4-97c5-fc900bf7b464"],
    ["12063760019269155007", "a5b64eba-caa2-4336-9d0b-04a2e2eb9bc5"],
    ["14640892673165708323", "fa3e5132-30b4-4429-a2fe-3a13c7ca5bcf"],
    ["6788698777015378439", "0e09c354-9c63-4fca-af77-492a71ed2ab1"]
  ]
}
```

7. Receive the response from UID Mapper.
   - If UID Mapper returns 204, all is OK, you can return 204.
   - If UID Mapper returns anything else, return 500 with a text describing the error.

## Configuration

### Access to ID HASH (important)
The ID HASH component is protected by an Auth Middleware, which ensures that only authorized Data Vendors can initiate requests.
To maintain this security, 
the only IP address allowed to access ID HASH should be that of your Auth Middleware instance (see the Config File Example section)!


### Config File Example
```ini
[http]
httpServerListenAddr        = :8000
httpServerName              = MyGaru ID HASH
# allow only Auth Middleware IP to access!
httpAuthAllowedRemoteIPs    = 127.0.0.1


[pim]
# the address of your UID Mapper component instance
uidMapAddr = http://your.uidmap.instance
pimTimeout = 5m
```
