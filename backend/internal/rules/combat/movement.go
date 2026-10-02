package combat

// FeetPerSquare is the side of a map square: 5 ft, which the app shows as
// 1,5 m.
const FeetPerSquare = 5

// GridDistanceSquares is the distance between two squares counting a
// diagonal step as one square, like the SRD's grid (RN-21).
func GridDistanceSquares(fromCol, fromRow, toCol, toRow int) int {
	return max(abs(toCol-fromCol), abs(toRow-fromRow))
}

// GridDistanceFt is GridDistanceSquares in feet.
func GridDistanceFt(fromCol, fromRow, toCol, toRow int) int {
	return GridDistanceSquares(fromCol, fromRow, toCol, toRow) * FeetPerSquare
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
