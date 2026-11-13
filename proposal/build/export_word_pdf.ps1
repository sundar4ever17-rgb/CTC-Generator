$ErrorActionPreference = 'Stop'
$proposalDir = 'D:\CTC GENERATOR\proposal'
$wordApp = $null
$proposalDocument = $null
try {
    $wordApp = New-Object -ComObject Word.Application
    $wordApp.Visible = $false
    $wordApp.DisplayAlerts = 0
    $proposalDocument = $wordApp.Documents.Open((Join-Path $proposalDir 'ctc-generator-project-proposal.docx'), $false, $true)
    $proposalDocument.Repaginate()
    $proposalDocument.ExportAsFixedFormat((Join-Path $proposalDir 'ctc-generator-project-proposal.pdf'), 17)
    Write-Output ('Exported PDF. Pages: ' + $proposalDocument.ComputeStatistics(2))
} finally {
    if ($null -ne $proposalDocument) {
        $proposalDocument.Close(0)
        [void][System.Runtime.InteropServices.Marshal]::FinalReleaseComObject($proposalDocument)
    }
    if ($null -ne $wordApp) {
        $wordApp.Quit()
        [void][System.Runtime.InteropServices.Marshal]::FinalReleaseComObject($wordApp)
    }
}
