import csv
import zipfile
import xml.sax.saxutils

def make_xlsx(csv_path, xlsx_path):
    rows = []
    with open(csv_path, 'r', encoding='utf-8') as f:
        reader = csv.reader(f)
        for r in reader:
            rows.append(r)

    sheet_xml = ['<?xml version="1.0" encoding="UTF-8" standalone="yes"?>',
                 '<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">',
                 '<sheetData>']
    for r_idx, row in enumerate(rows, start=1):
        sheet_xml.append(f'<row r="{r_idx}">')
        for c_idx, cell in enumerate(row, start=1):
            col_letter = chr(64 + c_idx)
            val = xml.sax.saxutils.escape(cell)
            sheet_xml.append(f'<c r="{col_letter}{r_idx}" t="inlineStr"><is><t>{val}</t></is></c>')
        sheet_xml.append('</row>')
    sheet_xml.append('</sheetData></worksheet>')
    sheet_content = ''.join(sheet_xml).encode('utf-8')

    content_types = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
        '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
        '<Default Extension="xml" ContentType="application/xml"/>'
        '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
        '<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
        '</Types>'
    ).encode('utf-8')

    rels = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>'
        '</Relationships>'
    ).encode('utf-8')

    wb = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
        '<sheets><sheet name="Feature Parity Matrix" sheetId="1" r:id="rId1"/></sheets>'
        '</workbook>'
    ).encode('utf-8')

    wb_rels = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>'
        '</Relationships>'
    ).encode('utf-8')

    with zipfile.ZipFile(xlsx_path, 'w', zipfile.ZIP_DEFLATED) as zf:
        zf.writestr('[Content_Types].xml', content_types)
        zf.writestr('_rels/.rels', rels)
        zf.writestr('xl/_rels/workbook.xml.rels', wb_rels)
        zf.writestr('xl/workbook.xml', wb)
        zf.writestr('xl/worksheets/sheet1.xml', sheet_content)
    print('Generated XLSX at:', xlsx_path)

if __name__ == '__main__':
    make_xlsx('C:/Users/dnjst/Desktop/project/line/docs/FEATURE_STATUS_001_100.csv', 'C:/Users/dnjst/Desktop/project/line/docs/MESSENGER_LINE_FEATURE_PARITY_MATRIX.xlsx')
