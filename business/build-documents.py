"""Render the proposal and executive summary from their editable Markdown sources."""
from pathlib import Path
import html,json,re,os
from reportlab.platypus import SimpleDocTemplate,Paragraph,Spacer,PageBreak,Table,TableStyle,Image,KeepTogether
from reportlab.lib.styles import getSampleStyleSheet,ParagraphStyle
from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.lib.pagesizes import A4
from reportlab.graphics.charts.lineplots import LinePlot
from reportlab.graphics.shapes import Drawing, String, Line
from reportlab.graphics import renderPDF
import subprocess, shutil

ROOT=Path(__file__).resolve().parent
fontdir=Path(os.environ.get('RESTRINGING_FONT_DIR','/System/Library/Fonts/Supplemental'))
for name,file in [('Arial','Arial.ttf'),('Arial-Bold','Arial Bold.ttf'),('Arial-Italic','Arial Italic.ttf')]:
 pdfmetrics.registerFont(TTFont(name,str(fontdir/file)))
pdfmetrics.registerFontFamily('Arial',normal='Arial',bold='Arial-Bold',italic='Arial-Italic',boldItalic='Arial-Bold')
INK=colors.HexColor('#123C32'); MUTED=colors.HexColor('#52665B'); LIGHT=colors.HexColor('#EDF2E9'); LIME=colors.HexColor('#C8F36B')
styles=getSampleStyleSheet()
styles.add(ParagraphStyle(name='BodyCustom',fontName='Arial',fontSize=10.0,leading=14.2,textColor=colors.HexColor('#203129'),spaceAfter=9))
styles.add(ParagraphStyle(name='TitleCustom',fontName='Arial-Bold',fontSize=28,leading=33,textColor=INK,spaceAfter=16))
styles.add(ParagraphStyle(name='SectionCustom',fontName='Arial-Bold',fontSize=19,leading=24,textColor=INK,spaceAfter=15,keepWithNext=True))
styles.add(ParagraphStyle(name='SubCustom',fontName='Arial-Bold',fontSize=11.8,leading=16,textColor=INK,spaceBefore=10,spaceAfter=8,keepWithNext=True))
styles.add(ParagraphStyle(name='CellCustom',fontName='Arial',fontSize=8.5,leading=11.6,textColor=INK))
styles.add(ParagraphStyle(name='CellHead',parent=styles['CellCustom'],fontName='Arial-Bold',textColor=colors.white))
styles.add(ParagraphStyle(name='SourceCustom',parent=styles['BodyCustom'],fontSize=8.8,leading=12.1,spaceAfter=7))
WIDTH=A4[0]-100
GITHUB='https://github.com/daviddoherty10/Restringing.ie/blob/codex/business-proposal/business/'
def inline(s):
 s=html.escape(s)
 s=re.sub(r'`([^`]+)`',lambda m:'<font name="Arial">'+m[1]+'</font>',s)
 def link(m):
  label,url=m.group(1),html.unescape(m.group(2))
  if not url.startswith(('http://','https://')):url=GITHUB+url
  return '<link color="#28694B" href="'+html.escape(url,quote=True)+'"><u>'+label+'</u></link>'
 s=re.sub(r'\[([^\]]+)\]\(([^)]+)\)',link,s)
 s=re.sub(r'\*\*([^*]+)\*\*',r'<b>\1</b>',s)
 return s

def table(lines):
 rows=[[c.strip() for c in l.strip().strip('|').split('|')] for l in lines]
 rows=[r for r in rows if not all(re.fullmatch(r':?-+:?',c.replace(' ','')) for c in r)]
 n=len(rows[0]); assert all(len(r)==n for r in rows)
 if n==4: widths=[WIDTH*.29,WIDTH*.237,WIDTH*.237,WIDTH*.236]
 elif n==3:widths=[WIDTH*.26,WIDTH*.37,WIDTH*.37]
 elif n==2:widths=([WIDTH*.85,WIDTH*.15] if rows[0][0]=='Section' else [WIDTH*.74,WIDTH*.26] if 'cost' in rows[0][1].lower() else [WIDTH*.34,WIDTH*.66])
 elif n==5:widths=[WIDTH*.15,WIDTH*.27,WIDTH*.17,WIDTH*.18,WIDTH*.23]
 else:widths=[WIDTH/n]*n
 data=[[Paragraph(inline(c),styles['CellHead' if i==0 else 'CellCustom']) for c in row] for i,row in enumerate(rows)]
 t=Table(data,colWidths=widths,repeatRows=1,hAlign='LEFT')
 cmds=[('BACKGROUND',(0,0),(-1,0),INK),('VALIGN',(0,0),(-1,-1),'TOP'),('LEFTPADDING',(0,0),(-1,-1),7),('RIGHTPADDING',(0,0),(-1,-1),7),('TOPPADDING',(0,0),(-1,-1),5),('BOTTOMPADDING',(0,0),(-1,-1),5),('LINEBELOW',(0,-1),(-1,-1),.5,colors.HexColor('#C4D2C0'))]
 for i in range(1,len(rows)):
  if i%2:cmds.append(('BACKGROUND',(0,i),(-1,i),LIGHT))
 t.setStyle(TableStyle(cmds));return t

class ProposalDoc(SimpleDocTemplate):
 def afterFlowable(self,flow):
  if isinstance(flow,Paragraph) and flow.style.name in ['SectionCustom','TitleCustom']:
   title=flow.getPlainText();key=str(abs(hash(title)))
   self.canv.bookmarkPage(key);self.canv.addOutlineEntry(title,key,level=0,closed=False)

def decorate(c,doc,cover=False):
 p=doc.page
 c.saveState()
 if cover and p==1:
  c.setFillColor(INK);c.rect(0,0,*A4,fill=1,stroke=0)
  c.setStrokeColor(colors.HexColor('#31594A'));c.setLineWidth(.5)
  for x in range(330,750,22):c.line(x,100,x-150,470)
  for y in range(120,500,22):c.line(260,y,640,y+140)
  c.setFillColor(LIME);c.rect(50,724,45,4,fill=1,stroke=0)
 else:
  c.setFont('Arial-Bold',8);c.setFillColor(INK);c.drawString(50,A4[1]-30,'RESTRINGING.IE')
  c.setFont('Arial',8);c.setFillColor(MUTED);c.drawRightString(A4[0]-50,A4[1]-30,'Business planning | November 2026')
  c.setStrokeColor(colors.HexColor('#D9E3D3'));c.line(50,44,A4[0]-50,44)
  c.setFont('Arial',8);c.drawString(50,30,'Planning estimates. Commercial validation required.');c.drawRightString(A4[0]-50,30,str(p))
 c.restoreState()

def parse(text,source=False):
 lines=text.splitlines();story=[];i=0
 while i<len(lines):
  l=lines[i].strip()
  if not l:i+=1;continue
  if l=='<!-- pagebreak -->':story.append(PageBreak());i+=1;continue
  if l.startswith('|'):
   tbl=[]
   while i<len(lines) and lines[i].strip().startswith('|'):tbl.append(lines[i]);i+=1
   story.extend([table(tbl),Spacer(1,10)]);continue
  if l.startswith('!['):
   target=re.search(r'\(([^)]+)\)',l)[1];im=Image(str(ROOT/target),width=WIDTH,height=WIDTH*.32);story.extend([im,Spacer(1,8)]);i+=1;continue
  if l.startswith('#'):
   level=len(l)-len(l.lstrip('#'));title=l[level:].strip();sty='TitleCustom' if level==1 else 'SectionCustom' if level==2 else 'SubCustom'
   story.append(Paragraph(inline(title),styles[sty]));i+=1;continue
  para=[l];i+=1
  while i<len(lines) and lines[i].strip() and not lines[i].strip().startswith(('#','|','<!--','![')):
   # Keep numbered contents and bibliography entries separate.
   if re.match(r'^\d+\. ',lines[i].strip()):break
   para.append(lines[i].strip());i+=1
  story.append(Paragraph(inline(' '.join(para)),styles['SourceCustom' if source else 'BodyCustom']))
 return story

def chart():
 d=json.loads((ROOT/'financial-model/scenario-results.json').read_text())
 chart=Drawing(600,230)
 chart.add(String(5,213,'Annual net platform revenue',fontName='Arial-Bold',fontSize=13,fillColor=INK))
 chart.add(String(5,195,'Planning scenarios; after modelled VAT reserve',fontName='Arial',fontSize=9,fillColor=MUTED))
 lp=LinePlot();lp.x=65;lp.y=32;lp.width=510;lp.height=140
 lp.data=[[(r['year'],r['revenue']) for r in v['years']] for v in d.values()]
 lp.xValueAxis.valueMin=2027;lp.xValueAxis.valueMax=2031;lp.xValueAxis.valueSteps=list(range(2027,2032));lp.xValueAxis.labelTextFormat='%d'
 lp.yValueAxis.valueMin=0;lp.yValueAxis.valueMax=150000;lp.yValueAxis.valueStep=50000;lp.yValueAxis.labelTextFormat=lambda v:'€'+str(int(v/1000))+'k'
 for ax in [lp.xValueAxis,lp.yValueAxis]:
  ax.labels.fontName='Arial';ax.labels.fontSize=9;ax.strokeColor=colors.HexColor('#BFCFC2')
 palette=['#9B6B35','#28724F','#263F6D']
 for i,c in enumerate(palette):lp.lines[i].strokeColor=colors.HexColor(c);lp.lines[i].strokeWidth=2
 chart.add(lp)
 for i,(name,c) in enumerate(zip(d,palette)):
  x=90+i*165;chart.add(Line(x,10,x+18,10,strokeColor=colors.HexColor(c),strokeWidth=2));chart.add(String(x+24,7,name,fontName='Arial',fontSize=9,fillColor=INK))
 renderPDF.drawToFile(chart,str(ROOT/'assets/revenue-scenarios.pdf'))
 subprocess.run([shutil.which('pdftoppm'),'-scale-to','1800','-singlefile','-png',str(ROOT/'assets/revenue-scenarios.pdf'),str(ROOT/'assets/revenue-scenarios')],check=True)

def build():
 chart();text=(ROOT/'business-proposal.md').read_text();first,rest=text.split('<!-- pagebreak -->',1)
 cover=[]
 for content,size,color,gap in [('Restringing.ie',40,colors.white,30),('Business proposal',27,LIME,26),('An Irish marketplace for racket servicing',17,colors.white,12),('A staged route into professional stringing software',14,colors.HexColor('#DBE7D5'),60),('24 November 2026',11,colors.white,18),('Prepared for commercial partners, clubs, investors and enterprise advisers.',11,colors.white,20),('Pre-launch. Demand, revenue and partnerships remain unvalidated.',10,colors.HexColor('#DBE7D5'),10)]:
  cover.append(Paragraph(content,ParagraphStyle(name='cover'+str(size)+str(gap),fontName='Arial-Bold' if size>20 else 'Arial',fontSize=size,leading=size*1.3,textColor=color,spaceAfter=gap)))
 story=[Spacer(1,100),*cover,PageBreak(),*parse(rest),PageBreak(),*parse((ROOT/'sources.md').read_text(),source=True)]
 doc=ProposalDoc(str(ROOT/'business-proposal.pdf'),pagesize=A4,rightMargin=50,leftMargin=50,topMargin=58,bottomMargin=60,title='Restringing.ie business proposal',author='Restringing.ie',pageCompression=1)
 doc.build(story,onFirstPage=lambda c,d:decorate(c,d,True),onLaterPages=lambda c,d:decorate(c,d,True))
 doc2=ProposalDoc(str(ROOT/'executive-summary.pdf'),pagesize=A4,rightMargin=50,leftMargin=50,topMargin=58,bottomMargin=60,title='Restringing.ie executive summary',author='Restringing.ie')
 doc2.build(parse((ROOT/'executive-summary.md').read_text()),onFirstPage=decorate,onLaterPages=decorate)
 from pypdf import PdfReader
 for name in ['business-proposal.pdf','executive-summary.pdf']:
  reader=PdfReader(ROOT/name);assert all(len(p.extract_text().strip())>50 for p in reader.pages)
  print(name,len(reader.pages),'pages')
 assert len(PdfReader(ROOT/'executive-summary.pdf').pages)==2,'Executive summary must be two pages'

if __name__=='__main__':build()
