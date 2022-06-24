package pg

import (
	"database/sql/driver"
	"strconv"

	"github.com/jackc/pgtype"
	"github.com/pkg/errors"
	"gitlab.com/moderntoken/gateways/decimal"
)

// Decimal represents PostgresSQL type numeric (a.k.a. decimal)
type Decimal struct {
	Decimal decimal.Decimal
	Status  pgtype.Status
}

func (d *Decimal) Set(src interface{}) error {
	if src == nil {
		*d = Decimal{Status: pgtype.Null}
		return nil
	}

	switch value := src.(type) {
	case decimal.Decimal:
		*d = Decimal{Decimal: value, Status: pgtype.Present}
	case float32:
		*d = Decimal{Decimal: decimal.FloatToDecimal(float64(value)), Status: pgtype.Present}
	case float64:
		*d = Decimal{Decimal: decimal.FloatToDecimal(value), Status: pgtype.Present}
	case int8:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case uint8:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case int16:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case uint16:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case int32:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case uint32:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case int64:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case uint64:
		// uint64 could be greater than int64 so convert to string then to decimal
		dec := decimal.ParseDecimal(strconv.FormatUint(value, 10))
		*d = Decimal{Decimal: dec, Status: pgtype.Present}
	case int:
		*d = Decimal{Decimal: decimal.NewDecimal(int64(value), 0), Status: pgtype.Present}
	case uint:
		// uint could be greater than int64 so convert to string then to decimal
		dec := decimal.ParseDecimal(strconv.FormatUint(uint64(value), 10))
		*d = Decimal{Decimal: dec, Status: pgtype.Present}
	case string:
		dec := decimal.ParseDecimal(value)
		*d = Decimal{Decimal: dec, Status: pgtype.Present}
	default:
		// If all else fails see if pgtype.Numeric can handle it. If so, translate through that.
		num := &pgtype.Numeric{}
		if err := num.Set(value); err != nil {
			return errors.Errorf("cannot convert %v to Numeric", value)
		}

		buf, err := num.EncodeText(nil, nil)
		if err != nil {
			return errors.Errorf("cannot convert %v to Numeric", value)
		}

		dec := decimal.ParseDecimal(string(buf))
		*d = Decimal{Decimal: dec, Status: pgtype.Present}
	}

	return nil
}

func (d *Decimal) Get() interface{} {
	switch d.Status {
	case pgtype.Present:
		return d.Decimal
	case pgtype.Null:
		return nil
	default:
		return d.Status
	}
}

func (d *Decimal) AssignTo(dst interface{}) error {
	switch d.Status {
	case pgtype.Present:
		switch v := dst.(type) {
		case *decimal.Decimal:
			*v = d.Decimal
		case *float32:
			f := d.Decimal.Float()
			*v = float32(f)
		case *float64:
			f := d.Decimal.Float()
			*v = f
		case *int:
			n, err := strconv.ParseInt(d.Decimal.String(), 10, strconv.IntSize)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = int(n)
		case *int8:
			n, err := strconv.ParseInt(d.Decimal.String(), 10, 8)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = int8(n)
		case *int16:
			n, err := strconv.ParseInt(d.Decimal.String(), 10, 16)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = int16(n)
		case *int32:
			n, err := strconv.ParseInt(d.Decimal.String(), 10, 32)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = int32(n)
		case *int64:
			n, err := strconv.ParseInt(d.Decimal.String(), 10, 64)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = n
		case *uint:
			n, err := strconv.ParseUint(d.Decimal.String(), 10, strconv.IntSize)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = uint(n)
		case *uint8:
			n, err := strconv.ParseUint(d.Decimal.String(), 10, 8)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = uint8(n)
		case *uint16:
			n, err := strconv.ParseUint(d.Decimal.String(), 10, 16)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = uint16(n)
		case *uint32:
			n, err := strconv.ParseUint(d.Decimal.String(), 10, 32)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = uint32(n)
		case *uint64:
			n, err := strconv.ParseUint(d.Decimal.String(), 10, 64)
			if err != nil {
				return errors.Errorf("cannot convert %v to %T", dst, *v)
			}
			*v = n
		default:
			if nextDst, retry := pgtype.GetAssignToDstType(dst); retry {
				return d.AssignTo(nextDst)
			}
		}
	case pgtype.Null:
		return pgtype.NullAssignTo(dst)
	}

	return nil
}

func (d *Decimal) DecodeText(_ *pgtype.ConnInfo, src []byte) error {
	if src == nil {
		*d = Decimal{Status: pgtype.Null}
		return nil
	}

	dec := decimal.ParseDecimal(string(src))
	*d = Decimal{Decimal: dec, Status: pgtype.Present}
	return nil
}

func (d *Decimal) DecodeBinary(ci *pgtype.ConnInfo, src []byte) error {
	if src == nil {
		*d = Decimal{Status: pgtype.Null}
		return nil
	}

	// For now at least, implement this in terms of pgtype.Numeric

	num := &pgtype.Numeric{}
	if err := num.DecodeBinary(ci, src); err != nil {
		return err
	}

	buf, err := num.EncodeText(ci, nil)
	if err != nil {
		return err
	}

	dec := decimal.ParseDecimal(string(buf))
	*d = Decimal{Decimal: dec, Status: pgtype.Present}
	return nil
}

func (d *Decimal) EncodeText(_ *pgtype.ConnInfo, buf []byte) ([]byte, error) {
	switch d.Status {
	case pgtype.Null:
		return nil, nil
	case pgtype.Undefined:
		return nil, errors.New("cannot encode status undefined")
	}

	return append(buf, d.Decimal.String()...), nil
}

func (d *Decimal) EncodeBinary(ci *pgtype.ConnInfo, buf []byte) ([]byte, error) {
	switch d.Status {
	case pgtype.Null:
		return nil, nil
	case pgtype.Undefined:
		return nil, errors.New("cannot encode status undefined")
	}

	// For now at least, implement this in terms of pgtype.Numeric
	num := &pgtype.Numeric{}
	if err := num.DecodeText(ci, []byte(d.Decimal.String())); err != nil {
		return nil, err
	}

	return num.EncodeBinary(ci, buf)
}

// Scan implements the database/sql Scanner interface.
func (d *Decimal) Scan(src interface{}) error {
	if src == nil {
		*d = Decimal{Status: pgtype.Null}
		return nil
	}

	switch src := src.(type) {
	case float64:
		*d = Decimal{Decimal: decimal.FloatToDecimal(src), Status: pgtype.Present}
		return nil
	case string:
		return d.DecodeText(nil, []byte(src))
	case []byte:
		return d.DecodeText(nil, src)
	}

	return errors.Errorf("cannot scan %T", src)
}

// Value implements the database/sql/driver Valuer interface.
func (d *Decimal) Value() (driver.Value, error) {
	switch d.Status {
	case pgtype.Present:
		return d.Decimal.Value()
	case pgtype.Null:
		return nil, nil
	default:
		return nil, errors.New("cannot encode status undefined")
	}
}
