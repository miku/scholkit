package pubmed

import "encoding/xml"

type Article struct {
	XMLName         xml.Name `xml:"PubmedArticle"`
	Text            string   `xml:",chardata"`
	MedlineCitation struct {
		Text           string `xml:",chardata"`
		Status         string `xml:"Status,attr"`
		Owner          string `xml:"Owner,attr"`
		IndexingMethod string `xml:"IndexingMethod,attr"`
		PMID           struct {
			Text    string `xml:",chardata"`
			Version string `xml:"Version,attr"`
		} `xml:"PMID"`
		DateCompleted struct {
			Text  string `xml:",chardata"`
			Year  string `xml:"Year"`
			Month string `xml:"Month"`
			Day   string `xml:"Day"`
		} `xml:"DateCompleted"`
		DateRevised struct {
			Text  string `xml:",chardata"`
			Year  string `xml:"Year"`
			Month string `xml:"Month"`
			Day   string `xml:"Day"`
		} `xml:"DateRevised"`
		Article struct {
			Text     string `xml:",chardata"`
			PubModel string `xml:"PubModel,attr"`
			Journal  struct {
				Text string `xml:",chardata"`
				ISSN struct {
					Text     string `xml:",chardata"`
					IssnType string `xml:"IssnType,attr"`
				} `xml:"ISSN"`
				JournalIssue struct {
					Text        string `xml:",chardata"`
					CitedMedium string `xml:"CitedMedium,attr"`
					Volume      string `xml:"Volume"`
					Issue       string `xml:"Issue"`
					PubDate     struct {
						Text        string `xml:",chardata"`
						Year        string `xml:"Year"`
						Month       string `xml:"Month"`
						Day         string `xml:"Day"`
						MedlineDate string `xml:"MedlineDate"`
						Season      string `xml:"Season"`
					} `xml:"PubDate"`
				} `xml:"JournalIssue"`
				Title           string `xml:"Title"`
				ISOAbbreviation string `xml:"ISOAbbreviation"`
			} `xml:"Journal"`
			ArticleTitle struct {
				Text string `xml:",chardata"`
				Sub  string `xml:"sub"`
			} `xml:"ArticleTitle"`
			Pagination struct {
				Text       string `xml:",chardata"`
				MedlinePgn string `xml:"MedlinePgn"`
			} `xml:"Pagination"`
			AuthorList struct {
				Text       string `xml:",chardata"`
				CompleteYN string `xml:"CompleteYN,attr"`
				Author     []struct {
					Text            string `xml:",chardata"`
					ValidYN         string `xml:"ValidYN,attr"`
					LastName        string `xml:"LastName"`
					ForeName        string `xml:"ForeName"`
					Initials        string `xml:"Initials"`
					Suffix          string `xml:"Suffix"`
					AffiliationInfo struct {
						Text        string `xml:",chardata"`
						Affiliation string `xml:"Affiliation"`
					} `xml:"AffiliationInfo"`
					CollectiveName string `xml:"CollectiveName"`
				} `xml:"Author"`
			} `xml:"AuthorList"`
			Language  []string `xml:"Language"`
			GrantList struct {
				Text       string `xml:",chardata"`
				CompleteYN string `xml:"CompleteYN,attr"`
				Grant      []struct {
					Text    string `xml:",chardata"`
					GrantID string `xml:"GrantID"`
					Acronym string `xml:"Acronym"`
					Agency  string `xml:"Agency"`
					Country string `xml:"Country"`
				} `xml:"Grant"`
			} `xml:"GrantList"`
			PublicationTypeList struct {
				Text            string `xml:",chardata"`
				PublicationType []struct {
					Text string `xml:",chardata"`
					UI   string `xml:"UI,attr"`
				} `xml:"PublicationType"`
			} `xml:"PublicationTypeList"`
			Abstract struct {
				Text         string `xml:",chardata"`
				AbstractText []struct {
					Text        string `xml:",chardata"`
					Label       string `xml:"Label,attr"`
					NlmCategory string `xml:"NlmCategory,attr"`
				} `xml:"AbstractText"`
			} `xml:"Abstract"`
			VernacularTitle string `xml:"VernacularTitle"`
			ELocationID     []struct {
				Text    string `xml:",chardata"`
				EIdType string `xml:"EIdType,attr"`
				ValidYN string `xml:"ValidYN,attr"`
			} `xml:"ELocationID"`
			DataBankList struct {
				Text       string `xml:",chardata"`
				CompleteYN string `xml:"CompleteYN,attr"`
				DataBank   struct {
					Text                string `xml:",chardata"`
					DataBankName        string `xml:"DataBankName"`
					AccessionNumberList struct {
						Text            string   `xml:",chardata"`
						AccessionNumber []string `xml:"AccessionNumber"`
					} `xml:"AccessionNumberList"`
				} `xml:"DataBank"`
			} `xml:"DataBankList"`
		} `xml:"Article"`
		MedlineJournalInfo struct {
			Text        string `xml:",chardata"`
			Country     string `xml:"Country"`
			MedlineTA   string `xml:"MedlineTA"`
			NlmUniqueID string `xml:"NlmUniqueID"`
			ISSNLinking string `xml:"ISSNLinking"`
		} `xml:"MedlineJournalInfo"`
		ChemicalList struct {
			Text     string `xml:",chardata"`
			Chemical []struct {
				Text            string `xml:",chardata"`
				RegistryNumber  string `xml:"RegistryNumber"`
				NameOfSubstance struct {
					Text string `xml:",chardata"`
					UI   string `xml:"UI,attr"`
				} `xml:"NameOfSubstance"`
			} `xml:"Chemical"`
		} `xml:"ChemicalList"`
		CitationSubset  string `xml:"CitationSubset"`
		MeshHeadingList struct {
			Text        string `xml:",chardata"`
			MeshHeading []struct {
				Text           string `xml:",chardata"`
				DescriptorName struct {
					Text         string `xml:",chardata"`
					UI           string `xml:"UI,attr"`
					MajorTopicYN string `xml:"MajorTopicYN,attr"`
					Type         string `xml:"Type,attr"`
				} `xml:"DescriptorName"`
				QualifierName []struct {
					Text         string `xml:",chardata"`
					UI           string `xml:"UI,attr"`
					MajorTopicYN string `xml:"MajorTopicYN,attr"`
				} `xml:"QualifierName"`
			} `xml:"MeshHeading"`
		} `xml:"MeshHeadingList"`
		CommentsCorrectionsList struct {
			Text                string `xml:",chardata"`
			CommentsCorrections []struct {
				Text      string `xml:",chardata"`
				RefType   string `xml:"RefType,attr"`
				RefSource string `xml:"RefSource"`
				PMID      struct {
					Text    string `xml:",chardata"`
					Version string `xml:"Version,attr"`
				} `xml:"PMID"`
				Note string `xml:"Note"`
			} `xml:"CommentsCorrections"`
		} `xml:"CommentsCorrectionsList"`
		NumberOfReferences string `xml:"NumberOfReferences"`
		OtherID            []struct {
			Text   string `xml:",chardata"`
			Source string `xml:"Source,attr"`
		} `xml:"OtherID"`
		PersonalNameSubjectList struct {
			Text                string `xml:",chardata"`
			PersonalNameSubject []struct {
				Text     string `xml:",chardata"`
				LastName string `xml:"LastName"`
				ForeName string `xml:"ForeName"`
				Initials string `xml:"Initials"`
				Suffix   string `xml:"Suffix"`
			} `xml:"PersonalNameSubject"`
		} `xml:"PersonalNameSubjectList"`
		OtherAbstract struct {
			Text         string `xml:",chardata"`
			Type         string `xml:"Type,attr"`
			Language     string `xml:"Language,attr"`
			AbstractText string `xml:"AbstractText"`
		} `xml:"OtherAbstract"`

		KeywordList struct {
			Text    string `xml:",chardata"`
			Owner   string `xml:"Owner,attr"`
			Keyword []struct {
				Text         string `xml:",chardata"`
				MajorTopicYN string `xml:"MajorTopicYN,attr"`
			} `xml:"Keyword"`
		} `xml:"KeywordList"`
		GeneralNote []struct {
			Text  string `xml:",chardata"`
			Owner string `xml:"Owner,attr"`
		} `xml:"GeneralNote"`
		SpaceFlightMission []string `xml:"SpaceFlightMission"`
	} `xml:"MedlineCitation"`
	PubmedData struct {
		Text    string `xml:",chardata"`
		History struct {
			Text          string `xml:",chardata"`
			PubMedPubDate []struct {
				Text      string `xml:",chardata"`
				PubStatus string `xml:"PubStatus,attr"`
				Year      string `xml:"Year"`
				Month     string `xml:"Month"`
				Day       string `xml:"Day"`
				Hour      string `xml:"Hour"`
				Minute    string `xml:"Minute"`
			} `xml:"PubMedPubDate"`
		} `xml:"History"`
		PublicationStatus string `xml:"PublicationStatus"`
		ArticleIdList     struct {
			Text      string `xml:",chardata"`
			ArticleId []struct {
				Text   string `xml:",chardata"`
				IdType string `xml:"IdType,attr"`
			} `xml:"ArticleId"`
		} `xml:"ArticleIdList"`
		ReferenceList struct {
			Text      string `xml:",chardata"`
			Reference []struct {
				Text          string `xml:",chardata"`
				Citation      string `xml:"Citation"`
				ArticleIdList struct {
					Text      string `xml:",chardata"`
					ArticleId struct {
						Text   string `xml:",chardata"`
						IdType string `xml:"IdType,attr"`
					} `xml:"ArticleId"`
				} `xml:"ArticleIdList"`
			} `xml:"Reference"`
		} `xml:"ReferenceList"`
	} `xml:"PubmedData"`
}

func (doc *Article) ReleaseDate() string {
	date := doc.MedlineCitation.Article.Journal.JournalIssue.PubDate
	switch {
	case date.Year != "" && date.Month != "" && date.Day != "":
		return date.Year + "-" + date.Month + "-" + date.Day
	case date.Year != "" && date.Month != "":
		return date.Year + "-" + date.Month
	case date.Year != "" && date.Month == "":
		return date.Year
	default:
		return ""
	}
}

// PubmedArticleSet was generated 2024-03-15 13:17:34 by tir on k9 with zek 0.1.22.
type PubmedArticleSet struct {
	XMLName xml.Name `xml:"PubmedArticleSet"`
	Text    string   `xml:",chardata"`
	Article Article  `xml:"PubmedArticle"`
}
