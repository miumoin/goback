package main

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"
    "crypto/md5"
    "encoding/hex"
    _ "github.com/go-sql-driver/mysql"
)

// Global variables
var domain string
var access_key string
var systemid int 
var user_id int
var (
    data = make(map[string]map[string]interface{})
    mu   sync.Mutex
)

//database config
var db *sql.DB
var db_host string = "localhost"
var db_user string = "root"
var db_pass string = "int3gritY"
var db_name string = "goback"

//Some struct(s)
type Block struct {
    ID          int
    Type        string
    Title       string
    Content     string
    Author      int
    Slug        string
    Parent      int
    CreatedAt   time.Time
    ModifiedAt  time.Time
    Permission  string
    Metas       map[string]string
    Status      int
    Children    []Block
}

func main() {
    // Handler function for /
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        
        // Setting CORS headers
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Vuedoo-Access-Key, X-Vuedoo-Domain")
        w.Header().Set("Access-Control-Allow-Origin", "*")


        // Process preflight request, don't do any db operations
        if r.Method == http.MethodOptions {
            //do nothing
            return
        } else {
            fmt.Println("Flight")
            //fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)

            /*
            ** Connect database
            */
            // MySQL connection string
            dsn := db_user + ":" + db_pass + "@tcp(" + db_host + ")/" + db_name

            // Connect to MySQL database
            var err error
            db, err = sql.Open("mysql", dsn)
            if err != nil {
                fmt.Println("Error connecting to database:", err)
                return
            }
            defer db.Close()
            /*
            ** Datbase connected
            */

            url := r.URL.Path
            parts := strings.Split(url, "/")

            // Filter out empty parts
            var filteredParts []string
            for _, part := range parts[1:] {
                if part != "" {
                    filteredParts = append(filteredParts, part)
                }
            }

            // Construct function name
            functionName := strings.Join(filteredParts, "_")
            //fmt.Fprintf(w, "Func name: %s\n", functionName)

            // Reading headers and setting global variables
            domain = r.Header.Get("X-Vuedoo-Domain")
            access_key = r.Header.Get("X-Vuedoo-Access-Key")
            system_id, system_id_err := get_system_id( domain )
            if system_id_err != nil {
                fmt.Println("Error:", system_id_err)
                return
            }

            user_id, user_id_err := get_user_id( system_id, access_key )

            if user_id_err != nil {
                fmt.Println("Error:", user_id_err)
                return
            }

            fmt.Println("System ID:", system_id)
            fmt.Println("User ID:", user_id)

            /*fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
            fmt.Fprintf(w, "Domain: %s\n", domain)
            fmt.Fprintf(w, "Access Key: %s\n", access_key)
            fmt.Fprintf(w, "User id:", user_id)*/

            // Reading GET parameters
            getParams := r.URL.Query()

           // Ensure that only one goroutine can access or modify the 'data' map at a time.
            mu.Lock()
            defer mu.Unlock()

            // Initialize the sub-maps if they don't exist.
            if data["get"] == nil {
                data["get"] = make(map[string]interface{})
            }
            if data["post"] == nil {
                data["post"] = make(map[string]interface{})
            }

            // Extract GET parameters.
            for key, values := range getParams {
                // Assuming we just want to store the first value for each key.
                if len(values) > 0 {
                    data["get"][key] = values[0]
                }
            }

            // Extract POST parameters.
            if err := r.ParseForm(); err == nil {
                for key, values := range r.Form {
                    // Assuming we just want to store the first value for each key.
                    if len(values) > 0 {
                        data["post"][key] = values[0]
                    }
                }
            }

            // Response to indicate success.
            w.Write([]byte("Parameters stored successfully."))

            arr := app( functionName )

            jsonData, err := json.Marshal(arr)
            if err != nil {
                fmt.Println("Error:", err)
                return
            }

            fmt.Fprintf(w, "%s", jsonData)
        }

    })

    fmt.Println("Server listening on port 8081...")
    http.ListenAndServe(":8081", nil)
}

/*
    Common utility functions
*/
func get_system_id( domain string ) (int, error) {
    var id int

    // Check if a row with the given domain exists
    err := db.QueryRow("SELECT id FROM systems WHERE domain=?", domain).Scan(&id)
    switch {
    case err == sql.ErrNoRows:
        // If the row doesn't exist, insert it
        stmt, err := db.Prepare("INSERT INTO systems (subdomain, domain, status) VALUES (?, ?, ?)")
        if err != nil {
            return 0, fmt.Errorf("error preparing statement: %v", err)
        }
        defer stmt.Close()

        res, err := stmt.Exec("", domain, 0)
        if err != nil {
            return 0, fmt.Errorf("error inserting row: %v", err)
        }

        // Retrieve the inserted row id
        id, err := res.LastInsertId()
        if err != nil {
            return 0, fmt.Errorf("error retrieving last inserted id: %v", err)
        }

        return int(id), nil

    case err != nil:
        return 0, fmt.Errorf("error querying database: %v", err)

    default:
        return id, nil
    }
}

func get_user_id(system_id int, access_key string) (int, error) {
    var user_id int

    // Check if systemID and accessKey are not empty
    if system_id != 0 && access_key != "" {
        // Prepare SQL statement
        stmt, err := db.Prepare("SELECT id FROM users WHERE system_id=? AND access_key=?")
        if err != nil {
            return 0, fmt.Errorf("error preparing statement: %v", err)
        }
        defer stmt.Close()

        // Execute query
        rows, err := stmt.Query(system_id, access_key)
        if err != nil {
            return 0, fmt.Errorf("error executing query: %v", err)
        }
        defer rows.Close()

        // Fetch result
        if rows.Next() {
            if err := rows.Scan(&user_id); err != nil {
                return 0, fmt.Errorf("error scanning row: %v", err)
            }
            return user_id, nil
        }
    }

    // Return false if no user ID found
    return 0, nil
}

func get_meta(parent string, parent_id int, key string) (string, error) {
    var metaValue string

    // Prepare SQL statement
    stmt, err := db.Prepare("SELECT meta_value FROM metas WHERE parent=? AND parent_id=? AND meta_key=? AND status>0")
    if err != nil {
        return "", fmt.Errorf("error preparing statement: %v", err)
    }
    defer stmt.Close()

    // Execute query
    rows, err := stmt.Query(parent, parent_id, key)
    if err != nil {
        return "", fmt.Errorf("error executing query: %v", err)
    }
    defer rows.Close()

    // Fetch result
    if rows.Next() {
        if err := rows.Scan(&metaValue); err != nil {
            return "", fmt.Errorf("error scanning row: %v", err)
        }
        return metaValue, nil
    }

    // Return false if no meta value found
    return "", nil
}

func add_meta(parent string, parent_id int, meta_key string, meta_value interface{}) error {
    // Convert meta value to JSON if it's an array
    var metaValueJSON string
    if arr, ok := meta_value.([]interface{}); ok {
        metaValueBytes, err := json.Marshal(arr)
        if err != nil {
            return fmt.Errorf("error marshaling meta value to JSON: %v", err)
        }
        metaValueJSON = string(metaValueBytes)
    } else {
        metaValueJSON = fmt.Sprintf("%v", meta_value)
    }

    // Check if meta exists
    existingMeta, err := get_meta(parent, parent_id, meta_key)
    if err != nil {
        return fmt.Errorf("error checking existing meta: %v", err)
    }

    // Prepare SQL statement
    var stmt *sql.Stmt
    if existingMeta == "" {
        stmt, err = db.Prepare("INSERT INTO metas (parent, parent_id, meta_key, meta_value, status) VALUES (?, ?, ?, ?, '1')")
        if err != nil {
            return fmt.Errorf("error preparing insert statement: %v", err)
        }
    } else {
        stmt, err = db.Prepare("UPDATE metas SET meta_value=?, status=1 WHERE parent=? AND parent_id=? AND meta_key=?")
        if err != nil {
            return fmt.Errorf("error preparing update statement: %v", err)
        }
    }
    defer stmt.Close()

    // Execute statement
    if existingMeta == "" {
        _, err = stmt.Exec(parent, parent_id, meta_key, metaValueJSON)
    } else {
        _, err = stmt.Exec(metaValueJSON, parent, parent_id, meta_key)
    }
    if err != nil {
        return fmt.Errorf("error executing statement: %v", err)
    }

    return nil
}

func add_block(user_id int, block map[string]interface{}, slug string) ([]Block, error) {
    var blocks []Block

    if user_id <= 0 {
        return nil, fmt.Errorf("invalid user ID")
    }

    if slug == "" {
        slug = fmt.Sprintf("%d", time.Now().UnixNano())
    }

    // Check if block with slug and author exists
    var existing_block_id int
    err := db.QueryRow("SELECT id FROM blocks WHERE slug=? AND author=?", slug, user_id).Scan(&existing_block_id)
    switch {
    case err == sql.ErrNoRows:
        // Insert new block
        stmt, err := db.Prepare("INSERT INTO blocks (type, title, content, author, slug, parent, created_at, modified_at, permission, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, '1')")
        if err != nil {
            return nil, fmt.Errorf("error preparing insert statement: %v", err)
        }
        defer stmt.Close()

        _, err = stmt.Exec(block["type"], block["title"], block["content"], user_id, slug, block["parent"], time.Now(), time.Now(), block["permission"])
        if err != nil {
            return nil, fmt.Errorf("error executing insert statement: %v", err)
        }
    case err != nil:
        return nil, fmt.Errorf("error checking existing block: %v", err)
    default:
        // Update existing block
        stmt, err := db.Prepare("UPDATE blocks SET title=?, content=?, modified_at=? WHERE slug=?")
        if err != nil {
            return nil, fmt.Errorf("error preparing update statement: %v", err)
        }
        defer stmt.Close()

        _, err = stmt.Exec(block["title"], block["content"], time.Now(), slug)
        if err != nil {
            return nil, fmt.Errorf("error executing update statement: %v", err)
        }
    }

    // Retrieve the updated block
    rows, err := db.Query("SELECT id, type, title, content, author, slug, parent, created_at, modified_at, permission FROM blocks WHERE slug=? AND status>0 ORDER BY id DESC LIMIT 1", slug)
    if err != nil {
        return nil, fmt.Errorf("error querying block: %v", err)
    }
    defer rows.Close()

    for rows.Next() {
        var b Block
        if err := rows.Scan(&b.ID, &b.Type, &b.Title, &b.Content, &b.Author, &b.Slug, &b.Parent, &b.CreatedAt, &b.ModifiedAt, &b.Permission); err != nil {
            return nil, fmt.Errorf("error scanning block row: %v", err)
        }
        blocks = append(blocks, b)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating block rows: %v", err)
    }

    if len(blocks) > 0 {
        return blocks, nil
    }
    return nil, nil
}

func get_blocks(user_id int, block_type string, page int, entries_per_page int, parent int) ([]Block, error) {
    x := (page - 1) * entries_per_page
    rows, err := db.Query("SELECT id, type, title, content, author, slug, parent, created_at, modified_at, permission, status FROM blocks WHERE author=? AND type=? AND status=1 ORDER BY id DESC LIMIT ?, ?", user_id, block_type, x, entries_per_page)
    if err != nil {
        return nil, fmt.Errorf("error querying blocks: %v", err)
    }
    defer rows.Close()

    var blocks []Block
    for rows.Next() {
        var b Block
        if err := rows.Scan(&b.ID, &b.Type, &b.Title, &b.Content, &b.Author, &b.Slug, &b.Parent, &b.CreatedAt, &b.ModifiedAt, &b.Permission, &b.Status); err != nil {
            return nil, fmt.Errorf("error scanning block row: %v", err)
        }
        blocks = append(blocks, b)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating block rows: %v", err)
    }

    if len(blocks) > 0 {
        return blocks, nil
    }
    return nil, nil
}

func get_block(user_id int, block_type string, id int, slug string, parent int) ([]Block, error) {
    query := "SELECT id, type, title, content, author, slug, parent, created_at, modified_at, permission, status FROM blocks WHERE status>0"
    var params []interface{}
    
    if parent == 0 {
        query += " AND author=?"
        params = append(params, user_id)
    }
    if block_type != "" {
        query += " AND type=?"
        params = append(params, block_type)
    }
    if id > 0 {
        query += " AND id=?"
        params = append(params, id)
    }
    if slug != "" {
        query += " AND slug=?"
        params = append(params, slug)
    }
    if parent > 0 {
        query += " AND parent=?"
        params = append(params, parent)
    }
    query += " ORDER BY id DESC"

    rows, err := db.Query(query, params...)
    if err != nil {
        return nil, fmt.Errorf("error querying blocks: %v", err)
    }
    defer rows.Close()

    var blocks []Block
    for rows.Next() {
        var b Block
        if err := rows.Scan(&b.ID, &b.Type, &b.Title, &b.Content, &b.Author, &b.Slug, &b.Parent, &b.CreatedAt, &b.ModifiedAt, &b.Permission, &b.Status); err != nil {
            return nil, fmt.Errorf("error scanning block row: %v", err)
        }
        b.Metas = get_metas(block_type, b.ID)
        b.Children, err = get_block(user_id, "", 0, "", b.ID)
        if err != nil {
            return nil, fmt.Errorf("error getting block children: %v", err)
        }
        blocks = append(blocks, b)
    }

    if len(blocks) > 0 {
        return blocks, nil
    }
    return nil, nil
}

func get_metas(parent string, parent_id int) (map[string]string) {
    metas := make(map[string]string)
    rows, err := db.Query("SELECT meta_key, meta_value FROM metas WHERE parent=? AND parent_id=?", parent, parent_id)
    if err != nil {
        return nil
    }
    defer rows.Close()

    for rows.Next() {
        var meta_key, meta_value string
        if err := rows.Scan(&meta_key, &meta_value); err != nil {
            return nil
        }
        metas[meta_key] = meta_value
    }

    if err := rows.Err(); err != nil {
        return nil
    }

    return metas
}

func saveMetas(data map[string]interface{}, user_id int) (string, error) {
    parent := data["parent"].(string)
    var parent_id int
    if parent == "user" {
        parent_id = user_id
    } else {
        parent_id = data["parent_id"].(int)
    }

    metas := data["metas"].(map[string]interface{})
    for key, value := range metas {
        if err := add_meta(parent, parent_id, key, value.(string)); err != nil {
            return "", fmt.Errorf("error adding meta: %v", err)
        }
    }

    status := "fail"
    if user_id > 0 {
        status = "success"
    }
    res := map[string]string{"status": status}
    jsonData, err := json.Marshal(res)
    if err != nil {
        return "", fmt.Errorf("error marshalling JSON: %v", err)
    }

    return string(jsonData), nil
}

func signup(data map[string]interface{}, user_id int) (string, error) {
    ret := map[string]interface{}{
        "status": "fail",
        "type":   "authentication",
    }
    onboarded := 0

    stmt, err := db.Prepare("INSERT INTO users ( system_id, email, password, access_key, privileges ) VALUES ( ?, ?, ?, ?, '[]' )")
    if err != nil {
        return "", fmt.Errorf("error preparing SQL statement: %v", err)
    }
    defer stmt.Close()

    email := data["email"].(string)
    hashed_password := GetMD5Hash([]byte(data["password"].(string)))
    hashed_access_key := GetMD5Hash([]byte(uniqid()))

    _, err = stmt.Exec(system_id, email, hashed_password, hashed_access_key)
    if err != nil {
        return "", fmt.Errorf("error executing SQL statement: %v", err)
    }

    stmt, err = db.Prepare("SELECT id, email, access_key FROM users WHERE email=? AND system_id=?")
    if err != nil {
        return "", fmt.Errorf("error preparing SQL statement: %v", err)
    }
    defer stmt.Close()

    var arr_id int
    var arr_email string
    var arr_access_key string

    err = stmt.QueryRow(email, system_id).Scan(&arr_id, &arr_email, &arr_access_key)
    if err != nil {
        return "", fmt.Errorf("error querying row: %v", err)
    }

    if arr_id != 0 {
        if err := add_meta("users", arr_id, "name", data["name"].(string)); err != nil {
            return "", fmt.Errorf("error adding meta: %v", err)
        }

        /*
        //Send send email validation email
        validationKey := md5.Sum([]byte(uniqid()))
        if err := addMeta("users", arr_id, "signin_email_validation_key", validationKey); err != nil {
            return "", fmt.Errorf("error adding meta: %v", err)
        }

        subject := strings.Replace(strings.Replace(strings.Replace(BASE, "http://", "", -1), "https://", "", -1), "www.", "", -1) + " email confirmation"
        message := fmt.Sprintf(`Hi %s,

Request Email Confirmation.

Let's make sure this email is active. Please click on the link below to validate your email address and confirm that you are the owner of this account.:

%s/login/?validate=true&user_id=%d&key=%x

Ignore if you didn't sign up.

This is an auto generated email, do not reply back.`, data["name"].(string), BASE, arr_id, validationKey)

        if err := cmail(arr_email, subject, message); err != nil {
            return "", fmt.Errorf("error sending email: %v", err)
        }
        */

        picture := get_meta("users", arr_id, "picture")
        ret = map[string]interface{}{
            "status": "success",
            "type":   "authentication",
            "params": map[string]interface{}{
                "access_key":      "", //$arr_access_key,
                "email_validated": 0,
                "name":            data["name"].(string),
                "picture":         picture,
                "onboarded":       onboarded,
            },
        }
    }
    jsonData, err := json.Marshal(ret)
    if err != nil {
        return "", fmt.Errorf("error marshalling JSON: %v", err)
    }

    return string(jsonData), nil
}

func GetMD5Hash(text string) string {
    hasher := md5.New()
    hasher.Write([]byte(text))
    return hex.EncodeToString(hasher.Sum(nil))
}
/*
    App specific functions
*/

func app( action string ) []interface{} {
    // Simulating different structures in the array based on the input string
    switch action {
        case "int":
            return []interface{}{1, 2, 3}
        case "hello_world":
            return []interface{}{"hello", "world"}
        case "map":
            return []interface{}{map[string]int{"a": 1, "b": 2}, map[string]int{"x": 10, "y": 20}}
        default:
            return []interface{}{map[string]string{"status": "fail", "reason": "action not found"}}
    }
}