(import "bits")

;; Entry is a (String, Any) pair.
(defc Entry (Product String Any))

;; entryKey returns the key for an Entry
(defl entryKey {ent: Entry} String
    (!field ent 0)
)

;; entryValue returns the value for an Entry
(defl entryValue {ent: Entry} Any
    (!field ent 1)
)

;; Namespace is an Array of entries, sorted by their key.
(defc Namespace (List Entry))

(defl findGteq {ns: Namespace, k: String, i: Size} Size
    (if (eq? (entryKey (!slot ns i)) k)
        i
        ((self)
            ns
            k 
            (bits.b32_add i (b32 1))
        )
    )
)

(defl find {ns: Namespace, k: String} Size
    (findGteq ns k (b32 0))
)

(defl get {ns: Namespace, k: String} Any
    (entryValue (!slot ns
        (find ns k)
    ))
)

(pub
    Namespace
    Entry
    get
    entryKey
    entryValue
)
