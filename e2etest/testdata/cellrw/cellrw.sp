(import "substrate")
(import "substrate/cells")

(defl main {env: substrate.Env} ()
    (let {
            cell: (substrate.getCell env "cell0")
            console: (substrate.getConsole env "console")
        }
        (do
            (cells.cas
                cell
                (!anyValueFrom "abcd")
                (!anyValueFrom "1234")
            )
            (substrate.consoleWrite
                console
                (!anyValueTo (cells.load cell) String)
            )
            {}
        )
    )
)

(def cell0 "abcd")

(pub main cell0)
