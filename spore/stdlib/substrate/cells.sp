(import "namespaces")
(import "substrate/cells")

(defl cellLoad {x: CellDev} Any
    (!input x)
)

(defl cellPut {x: CellDev, next: Any} ()
    (!output x next)
)

(defl cellCAS {x: CellDev, prev: Any, next: Any} Any
    (!interact x {prev next})
)

(defl getCell {env: namespaces.Namespace, k: String} cells.Cell
    (let {
        dev : (!anyValueTo (namespaces.get env k) CellDev)
    }
        (!post {
            (lambda {} Any
                (cellLoad dev)
            )
            (lambda {next: Any} ()
                (cellPut dev next)
            )
            (lambda {prev: Any, next: Any} Any
                (cellCAS dev prev next)
            )
        })
    )
)

(pub getCell)