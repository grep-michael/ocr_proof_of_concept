device type -> Manufacturer -> some data

data -> models, serial, 


sfp module, - > manufacturer from list, 
    model detection:
        manufacturer based regex
        line number in text

macbook -> A number, Serial

Drive -> capcity, Manufacturer, serial,

User Selects Device type

type DeviceParser interface { //laptop, sfp, drive
    Parse(text string) (*ParseResult, error)
    DeviceType() string
    Manufacturer() string
}
type ParseResult struct {
    DeviceType string
    Fields     map[string]string // "serial", "manufacturer", "model", etc.
    Raw        string            
}
type FieldExtractor interface {
    FieldName() string
    Extract(text string) (value string, err error)
}
type ManufacturerResolver interface { //for laptops; dell, hp, mac, drives; western digital, segate, sfp; inno light, eoptolink, aoi
    Resolve(text string) (manufacturer string, matched bool)
    KnownManufacturers() []string
}
