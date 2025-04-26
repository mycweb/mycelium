(import "testing")

(defl assertB32 {expected: B32, actual: B32} ()
    (if (!equal expected actual)
        {}
        (do (!panic actual) {})
    )
)

(defl TestAddB32 (testing.T) ()
    (assertB32 (b32 10) (b32_add (b32 3) (b32 7)))
    (assertB32 (b32 0) (b32_add (b32 0) (b32 0)))
)

(pub TestAddB32)