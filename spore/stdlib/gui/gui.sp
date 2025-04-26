(import "substrate")
(import "bits")

(defc Point (Product bits.B32 bits.B32))
(defc Rect (Product Point Point))

(defc Color (Product bits.B8 bits.B8 bits.B8 bits.B8))

(defl color {r: bits.B8, g: bits.B8, b: bits.B8, a: bits.B8} Color
    {r g b a}
)

(pub Color color)

(defc FillOp (Product
    Color
))

(defc FillShapeOp (Product
    Color
    Rect
))

(defc Op (Sum
    FillOp
))

(defl fill {color: Color} Op
    (!makeSum Op (b32 0) {color})
)

(pub Op fill)

(defc OpList (Port Bottom Bottom Op ()))

(defl append {ops: OpList, op: Op} ()
    (!interact ops op)
)

(pub OpList append)

(defc RenderProc (Lambda
    (Product substrate.Env OpList)
    ()
))

(defc GUI (Product
    RenderProc
))

(defl mkGUI {render: RenderProc} GUI
    { render }
)

(pub GUI mkGUI)