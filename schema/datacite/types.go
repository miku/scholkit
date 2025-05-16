package datacite

type Document struct {
	Attributes struct {
		CitationCount int64 `json:"citationCount",omitempty`
		Container     struct {
			FirstPage      string `json:"firstPage",omitempty`
			Identifier     string `json:"identifier",omitempty`
			IdentifierType string `json:"identifierType",omitempty`
			Issue          string `json:"issue",omitempty`
			LastPage       string `json:"lastPage",omitempty`
			Title          string `json:"title",omitempty`
			Type           string `json:"type",omitempty`
			Volume         string `json:"volume",omitempty`
		} `json:"container",omitempty`
		ContentUrl   []interface{} `json:"contentUrl",omitempty`
		Contributors []struct {
			Affiliation     []interface{} `json:"affiliation",omitempty`
			ContributorType string        `json:"contributorType",omitempty`
			FamilyName      string        `json:"familyName",omitempty`
			GivenName       string        `json:"givenName",omitempty`
			Name            string        `json:"name",omitempty`
			NameIdentifiers []struct {
				NameIdentifier       string `json:"nameIdentifier",omitempty`
				NameIdentifierScheme string `json:"nameIdentifierScheme",omitempty`
				SchemeUri            string `json:"schemeUri",omitempty`
			} `json:"nameIdentifiers",omitempty`
			NameType string `json:"nameType",omitempty`
		} `json:"contributors",omitempty`
		Created  string `json:"created",omitempty`
		Creators []struct {
			Affiliation     []interface{} `json:"affiliation",omitempty`
			FamilyName      string        `json:"familyName",omitempty`
			GivenName       string        `json:"givenName",omitempty`
			Name            string        `json:"name",omitempty`
			NameIdentifiers []interface{} `json:"nameIdentifiers",omitempty`
			NameType        string        `json:"nameType",omitempty`
		} `json:"creators",omitempty`
		Dates []struct {
			Date            string      `json:"date",omitempty`
			DateInformation interface{} `json:"dateInformation",omitempty`
			DateType        string      `json:"dateType",omitempty`
		} `json:"dates",omitempty`
		Descriptions []struct {
			Description     string `json:"description",omitempty`
			DescriptionType string `json:"descriptionType",omitempty`
			Lang            string `json:"lang",omitempty`
		} `json:"descriptions",omitempty`
		DOI               string   `json:"doi",omitempty`
		DownloadCount     int64    `json:"downloadCount",omitempty`
		Formats           []string `json:"formats",omitempty`
		FundingReferences []struct {
			FunderName string `json:"funderName",omitempty`
		} `json:"fundingReferences",omitempty`
		GeoLocations []struct {
			GeoLocationPlace string `json:"geoLocationPlace",omitempty`
		} `json:"geoLocations",omitempty`
		Identifiers []struct {
			Identifier     string `json:"identifier",omitempty`
			IdentifierType string `json:"identifierType",omitempty`
		} `json:"identifiers",omitempty`
		IsActive           bool        `json:"isActive",omitempty`
		Language           string      `json:"language",omitempty`
		MetadataVersion    int64       `json:"metadataVersion",omitempty`
		PartCount          int64       `json:"partCount",omitempty`
		PartOfCount        int64       `json:"partOfCount",omitempty`
		PublicationYear    int64       `json:"publicationYear",omitempty`
		Published          string      `json:"published",omitempty`
		Publisher          string      `json:"publisher",omitempty`
		Reason             interface{} `json:"reason",omitempty`
		ReferenceCount     int64       `json:"referenceCount",omitempty`
		Registered         string      `json:"registered",omitempty`
		RelatedIdentifiers []struct {
			RelatedIdentifier     string      `json:"relatedIdentifier",omitempty`
			RelatedIdentifierType string      `json:"relatedIdentifierType",omitempty`
			RelatedMetadataScheme interface{} `json:"relatedMetadataScheme",omitempty`
			RelationType          string      `json:"relationType",omitempty`
			ResourceTypeGeneral   interface{} `json:"resourceTypeGeneral",omitempty`
			SchemeType            interface{} `json:"schemeType",omitempty`
			SchemeUri             interface{} `json:"schemeUri",omitempty`
		} `json:"relatedIdentifiers",omitempty`
		RelatedItems []struct {
			PublicationYear       any `json:"publicationYear",omitempty`
			RelatedItemIdentifier struct {
				RelatedItemIdentifier     string `json:"relatedItemIdentifier",omitempty`
				RelatedItemIdentifierType string `json:"relatedItemIdentifierType",omitempty`
			} `json:"relatedItemIdentifier",omitempty`
			RelatedItemType string `json:"relatedItemType",omitempty`
			RelationType    string `json:"relationType",omitempty`
			Titles          any    `json:"title"`
			// Titles          []struct {
			// 	Title string `json:"title",omitempty`
			// } `json:"titles",omitempty`
		} `json:"relatedItems",omitempty`
		RightsList []struct {
			Rights string `json:"rights",omitempty`
		} `json:"rightsList",omitempty`
		SchemaVersion string `json:"schemaVersion",omitempty`
		Sizes         any    `json:"sizes,omitempty"`
		// Sizes         []string `json:"sizes",omitempty`
		Source   string `json:"source",omitempty`
		State    string `json:"state",omitempty`
		Subjects []struct {
			Subject string `json:"subject",omitempty`
		} `json:"subjects",omitempty`
		Titles []struct {
			Lang      string `json:"lang",omitempty`
			Title     string `json:"title",omitempty`
			TitleType string `json:"titleType",omitempty`
		} `json:"titles",omitempty`
		Types struct {
			Bibtex              string `json:"bibtex",omitempty`
			Citeproc            string `json:"citeproc",omitempty`
			ResourceType        string `json:"resourceType",omitempty`
			ResourceTypeGeneral string `json:"resourceTypeGeneral",omitempty`
			Ris                 string `json:"ris",omitempty`
			SchemaOrg           string `json:"schemaOrg",omitempty`
		} `json:"types",omitempty`
		Updated        string `json:"updated",omitempty`
		Url            string `json:"url",omitempty`
		Version        string `json:"version",omitempty`
		VersionCount   int64  `json:"versionCount",omitempty`
		VersionOfCount int64  `json:"versionOfCount",omitempty`
		ViewCount      int64  `json:"viewCount",omitempty`
	} `json:"attributes",omitempty`
	Id            string `json:"id",omitempty`
	Relationships struct {
		Client struct {
			Data struct {
				Id   string `json:"id",omitempty`
				Type string `json:"type",omitempty`
			} `json:"data",omitempty`
		} `json:"client",omitempty`
	} `json:"relationships",omitempty`
	Type string `json:"type",omitempty`
}
