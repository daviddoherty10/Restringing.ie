import fs from 'node:fs/promises';
import path from 'node:path';
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const {Workbook, SpreadsheetFile} = await import(require.resolve('@oai/artifact-tool'));
const root=path.dirname(new URL(import.meta.url).pathname);
const a=JSON.parse(await fs.readFile(path.join(root,'assumptions.json'),'utf8'));
const wb=Workbook.create();
const summary=wb.worksheets.add('Summary'), inputs=wb.worksheets.add('Assumptions'), build=wb.worksheets.add('Monthly build'), compare=wb.worksheets.add('Case comparison');
const money='"€"#,##0;("€"#,##0);"-"';
const num='#,##0;(#,##0);"-"';
const put=(s,cell,v)=>s.getRange(cell).values=[[v]];
const formula=(s,cell,v)=>s.getRange(cell).formulas=[[v]];
const header=(s,r,c='C',end='H')=>{s.getRange(`${c}${r}:${end}${r}`).format={fill:'#123C32',font:{color:'#FFFFFF',bold:true},rowHeight:27};};
for(const s of [summary,inputs,build,compare]){
 s.showGridLines=false;s.getRange('A1:AE110').format.font={name:'Arial',size:10,color:'#17211D'};
 s.getRange('A1:B110').format.columnWidth=2;
 s.getRange('C1:C110').format.columnWidth=38;
 s.getRange('D1:AE110').format.columnWidth=15;
 s.getRange('C1:AE110').format.rowHeight=22;
 s.tabColor=s===summary?'#123C32':s===inputs?'#648769':'#B9CABA';
}
put(summary,'C2','Restringing.ie financial plan');put(inputs,'C2','Assumptions');put(build,'C2','Monthly operating and cash model');put(compare,'C2','Saved case comparison');
for(const s of [summary,inputs,build,compare])s.getRange('C2').format.font={size:15,bold:true};
put(inputs,'C4','Case selected:');put(inputs,'D4',2);formula(inputs,'E4','=CHOOSE(D4,"Conservative","Base","Optimistic")');
inputs.getRange('D4').dataValidation={rule:{type:'whole',operator:'between',formula1:1,formula2:3}};
put(inputs,'C5','1 Conservative / 2 Base / 3 Optimistic');
for(const s of [summary,build]){put(s,'C4','Case selected:');formula(s,'D4',"='Assumptions'!E4");s.getRange('D4').format.font.color='#008000';}
put(summary,'C5','2027 launch assumption; all figures are forecasts, in euro. No opening cash or committed funding.');
put(compare,'C4','Snapshots at 17 November 2026; workbook edits do not refresh these figures.');
put(compare,'C5','Re-run build-model.mjs from assumptions.json to refresh case snapshots; update written documents separately.');
const globals=[
 ['Subscription gross monthly',a.prices.subscription,'Proposed price; founder handoff'],
 ['Event gross licence',a.prices.event,'Planning assumption; not a supplier price'],
 ['Pro tools gross monthly',a.prices.pro,'Separate professional product; planning assumption'],
 ['Annual partner fee gross',a.prices.partnerAnnual,'No signed partners; planning assumption'],
 ['Locker job fee gross',a.prices.lockerJob,'One full deposit/return cycle; planning assumption'],
 ['VAT reserve rate',a.costs.vatReserve,'Planning reserve; Revenue VAT thresholds in sources.md #20'],
 ['Processing percent of gross',a.costs.processorPercent,'Budget estimate; not a quoted Stripe tariff'],
 ['Processing fixed per transaction',a.costs.processorFixed,'Budget estimate; applied to all paid streams'],
 ['Variable cost per managed booking',a.costs.bookingVariable,'Notification/service budget; planning estimate'],
 ['Delivery cost per tournament',a.costs.eventDelivery,'Support/travel allowance; excludes stringing labour'],
 ['Cost per pro account per month',a.costs.proAccountMonthly,'Hosting/support budget; planning estimate'],
 ['Locker logistics per full job',a.costs.lockerJobVariable,'Consolidated route assumption, not a courier quote'],
 ['Locker site monthly cost',a.costs.lockerSiteMonthly,'Connectivity, maintenance, host/insurance budget'],
 ['Installed locker capex per site',a.costs.lockerCapex,'Illustrative estimate; get three quotations'],
 ['Startup before contingency',a.costs.startup,'Itemised in proposal, section 12'],
 ['Startup contingency rate',a.costs.startupContingency,'Planning estimate'],
 ['Minimum cash buffer',a.costs.cashBuffer,'Planning estimate; not existing cash']
];
header(inputs,7,'C','E');inputs.getRange('C7:E7').values=[['Common inputs','Value','Basis / source']];
const g={};globals.forEach(([label,v,note],i)=>{const r=8+i;g[label]=`'Assumptions'!$D$${r}`;inputs.getRange(`C${r}:E${r}`).values=[[label,v,note]];});
inputs.getRange('D8:D24').format.font.color='#0000FF';inputs.getRange('D8:D24').format.fill='#FFF7D6';
inputs.getRange('D13:D14').setNumberFormat('0.0%');inputs.getRange('D23').setNumberFormat('0.0%');
inputs.getRange('E1:E110').format.columnWidth=19;
inputs.getRange('J1:J110').format.columnWidth=75;
// Sources sit to the right of the common-input table so the period columns remain compact.
globals.forEach((row,i)=>{put(inputs,`J${8+i}`,row[2]);put(inputs,`E${8+i}`,'See column J');});
header(inputs,27);put(inputs,'C27','Monthly paid-plan seasonality');
a.seasonality.forEach((v,i)=>{put(inputs,`C${28+i}`,new Date(2027,i,1).toLocaleString('en-IE',{month:'short',timeZone:'UTC'}));put(inputs,`D${28+i}`,v);});
inputs.getRange('D28:D39').setNumberFormat('0.0%');inputs.getRange('D28:D39').format.font.color='#0000FF';
const keys=['grossAdds','monthlyChurn','eligibleShare','upgradeShare','jobsPerBusiness','events','proAccounts','partners','lockerSites','lockerJobsPerSite','monthlyOverhead','annualMarketing','annualPeople'];
const labels=['New stringing businesses / year','Monthly business churn','Share exceeding free allowance','Upgrade share of eligible businesses','Managed bookings / business / month','Tournament licences / year','Pro-only paying accounts, annual average','Annual commercial partners','Operating locker sites','Jobs / locker site / month','Fixed operating costs / month','Acquisition spend / year','Founder and contractor cash cost / year'];
const active={};let r=43;
for(let i=0;i<keys.length;i++){
 const k=keys[i]; active[k]=r;
 header(inputs,r);put(inputs,`C${r}`,labels[i]+' (active)');
 for(let y=0;y<5;y++){const c=String.fromCharCode(68+y);formula(inputs,`${c}${r}`,`=CHOOSE($D$4,${c}${r+1},${c}${r+2},${c}${r+3})`);}
 for(let sc=0;sc<3;sc++){put(inputs,`C${r+sc+1}`,a.scenarios[sc].name);for(let y=0;y<5;y++){const c=String.fromCharCode(68+y),val=a.scenarios[sc][k];put(inputs,`${c}${r+sc+1}`,Array.isArray(val)?val[y]:val);}}
 inputs.getRange(`D${r+1}:H${r+3}`).format.font.color='#0000FF';inputs.getRange(`D${r+1}:H${r+3}`).format.fill='#FFF7D6';
 if(['monthlyChurn','eligibleShare','upgradeShare'].includes(k))inputs.getRange(`D${r}:H${r+3}`).setNumberFormat('0.0%');
 r+=5;
}
put(inputs,'C41','Annual drivers');inputs.getRange('D41:H41').values=[[2027,2028,2029,2030,2031]];header(inputs,41);
inputs.freezePanes.freezeRows(6);
inputs.getRange('D4').format={fill:'#FFF7D6',font:{color:'#0000FF'}};inputs.getRange('E4').format.font.color='#008000';
put(inputs,'J28','Blue / pale yellow = editable inputs. Green = links from another sheet.');
put(inputs,'J30','Common prices include the VAT reserve. Costs include unrecovered input VAT.');
put(inputs,'J32','Pro accounts exclude marketplace subscribers; do not count bundled accounts twice.');
put(inputs,'J34','New sites open in January. Events spread evenly. Pro accounts are annual averages.');
put(inputs,'J36','Churn counts businesses leaving the platform, separate from seasonal paid months.');
put(inputs,'J38','All scenario drivers are estimates. Source links and research limitations: ../sources.md.');
const drive=(key,yearCell)=>`INDEX('Assumptions'!$D$${active[key]}:$H$${active[key]},1,${yearCell})`;
const cols=['Index','Year','Month','Opening businesses','New businesses','Lost businesses','Closing businesses','Average businesses','Paid equivalents','Managed bookings','Subscriptions net','Events net','Pro tools net','Partners net','Locker fees net','Total net revenue','Processing costs','Booking costs','Event costs','Pro costs','Locker costs','Fixed overhead','Acquisition spend','People cash cost','Operating surplus','Locker capex','Startup cash cost','Net cash change','Cumulative cash'];
build.getRange('C6:AE6').values=[cols];header(build,6,'C','AE');build.getRange('C6:AE6').format.wrapText=true;build.getRange('C6:AE6').format.rowHeight=40;build.getRange('C1:C70').format.columnWidth=10;
for(let i=0;i<60;i++){
 const n=7+i, y=`D${n}`,mo=`E${n}`,dx=k=>drive(k,`(${y}-2026)`), vr=g['VAT reserve rate'];
 const f={C:`=${i+1}`,D:`=2027+INT((C${n}-1)/12)`,E:`=MOD(C${n}-1,12)+1`,F:i?`=I${n-1}`:'=0',G:`=${dx('grossAdds')}/12`,H:`=F${n}*${dx('monthlyChurn')}`,I:`=F${n}+G${n}-H${n}`,J:`=(F${n}+I${n})/2`,K:`=J${n}*${dx('eligibleShare')}*${dx('upgradeShare')}*INDEX('Assumptions'!$D$28:$D$39,${mo})`,L:`=J${n}*${dx('jobsPerBusiness')}`,M:`=K${n}*${g['Subscription gross monthly']}/(1+${vr})`,N:`=${dx('events')}/12*${g['Event gross licence']}/(1+${vr})`,O:`=${dx('proAccounts')}*${g['Pro tools gross monthly']}/(1+${vr})`,P:`=${dx('partners')}/12*${g['Annual partner fee gross']}/(1+${vr})`,Q:`=${dx('lockerSites')}*${dx('lockerJobsPerSite')}*${g['Locker job fee gross']}/(1+${vr})`,R:`=SUM(M${n}:Q${n})`,S:`=R${n}*(1+${vr})*${g['Processing percent of gross']}+(K${n}+${dx('events')}/12+${dx('proAccounts')}+${dx('partners')}/12+${dx('lockerSites')}*${dx('lockerJobsPerSite')})*${g['Processing fixed per transaction']}`,T:`=L${n}*${g['Variable cost per managed booking']}`,U:`=${dx('events')}/12*${g['Delivery cost per tournament']}`,V:`=${dx('proAccounts')}*${g['Cost per pro account per month']}`,W:`=${dx('lockerSites')}*(${dx('lockerJobsPerSite')}*${g['Locker logistics per full job']}+${g['Locker site monthly cost']})`,X:`=${dx('monthlyOverhead')}`,Y:`=${dx('annualMarketing')}/12`,Z:`=${dx('annualPeople')}/12`,AA:`=R${n}-SUM(S${n}:Z${n})`,AB:`=IF(E${n}=1,MAX(0,${dx('lockerSites')}-${i<12?'0':drive('lockerSites',`(${y}-2027)`)})*${g['Installed locker capex per site']},0)`,AC:`=IF(C${n}=1,${g['Startup before contingency']}*(1+${g['Startup contingency rate']}),0)`,AD:`=AA${n}-SUM(AB${n}:AC${n})`,AE:i?`=AE${n-1}+AD${n}`:`=AD${n}`};
 for(const [c,v] of Object.entries(f)){formula(build,`${c}${n}`,v);build.getRange(`${c}${n}`).format.font.color=v.includes("'Assumptions'!")?'#008000':'#000000';}
}
build.getRange('D1:AE70').format.columnWidth=19;build.getRange('C1:C70').format.columnWidth=16;build.getRange('C6:AE6').format.horizontalAlignment='center';
build.getRange('F7:L66').setNumberFormat('0.0');build.getRange('M7:AE66').setNumberFormat(money);build.freezePanes.freezeRows(6);build.freezePanes.freezeColumns(5);
summary.getRange('D7:H7').values=[[2027,2028,2029,2030,2031]];put(summary,'C7','Annual forecasts');header(summary,7);
const labelsOut={8:'Closing active stringing businesses',9:'Average monthly paid equivalents',10:'Managed bookings (not new demand)',12:'Marketplace subscriptions',13:'Tournament licences',14:'Professional tools',15:'Commercial partnerships',16:'Locker service fees',17:'Total net revenue',19:'Variable service and processing costs',20:'Fixed operating costs',21:'Acquisition spend',22:'Founder and contractor cash costs',23:'Operating surplus before tax and D&A',25:'Locker capital expenditure',26:'Startup cash expenditure',27:'Net cash change',28:'Cumulative cash before funding',29:'Funding required incl. cash buffer'};
for(const [r,label] of Object.entries(labelsOut))put(summary,`C${r}`,label);
for(let y=0;y<5;y++){
 const c=String.fromCharCode(68+y),first=7+y*12,last=first+11,range=k=>`'Monthly build'!${k}${first}:${k}${last}`;
 for(const [r,k] of [[8,'I'],[9,'K'],[10,'L'],[12,'M'],[13,'N'],[14,'O'],[15,'P'],[16,'Q'],[17,'R'],[20,'X'],[21,'Y'],[22,'Z'],[23,'AA'],[25,'AB'],[26,'AC'],[27,'AD'],[28,'AE']])formula(summary,`${c}${r}`,[8,28].includes(r)?`='Monthly build'!${k}${last}`:r===9?`=AVERAGE(${range(k)})`:`=SUM(${range(k)})`);
 formula(summary,`${c}19`,`=SUM('Monthly build'!S${first}:W${last})`);
 formula(summary,`${c}29`,`=MAX(0,-MIN('Monthly build'!AE$7:AE${last}))+${g['Minimum cash buffer']}`);
}
summary.getRange('D8:H10').setNumberFormat(num);summary.getRange('D12:H29').setNumberFormat(money);
for(const r of [17,23,27,29]){summary.getRange(`C${r}:H${r}`).format.fill='#EAF0E7';summary.getRange(`C${r}:H${r}`).format.font.bold=true;}
put(summary,'C32','Cash planning basis');
put(summary,'C33','VAT reserve segregated immediately. No input VAT recovery, financing, corporation tax or debtor balances.');
put(summary,'C34','Operating surplus excludes capital costs and depreciation. Net cash deducts startup spend and locker purchases.');
put(summary,'C35','Negative cumulative cash is an unfunded requirement. No grant, investment or founder money is assumed.');
put(summary,'C36','Year 1 has no founder pay. Unpaid founder labour is a material economic cost.');
put(summary,'C37','Prices and all demand drivers are unvalidated. Public product status and sources: ../business-proposal.md.');
// One calculation build; calculate each case and save its comparison explicitly.
const snapshots={};
for(let sc=1;sc<=3;sc++){
 put(inputs,'D4',sc);wb.recalculate();
 const values={};for(const r of Object.keys(labelsOut))values[r]=summary.getRange(`D${r}:H${r}`).values[0];snapshots[a.scenarios[sc-1].name]=values;
}
let cr=8;
for(const [name,values] of Object.entries(snapshots)){
 put(compare,`C${cr}`,name);compare.getRange(`D${cr}:H${cr}`).values=[[2027,2028,2029,2030,2031]];header(compare,cr);
 for(const r of [8,9,17,23,27,29]){cr++;put(compare,`C${cr}`,labelsOut[r]);compare.getRange(`D${cr}:H${cr}`).values=[values[r]];compare.getRange(`D${cr}:H${cr}`).setNumberFormat([8,9].includes(r)?num:money);}
 cr+=3;
}
put(inputs,'D4',2);wb.recalculate();
const baseRevenue=summary.getRange('H17').values[0][0];
put(inputs,'D8',16);wb.recalculate();if(!(summary.getRange('H17').values[0][0]>baseRevenue))throw Error('Price change failed to update revenue');put(inputs,'D8',15);
// Zero eligible users must generate no subscription revenue, while other streams remain intact.
const baseEligibleRow=active.eligibleShare+2,original=inputs.getRange(`D${baseEligibleRow}:H${baseEligibleRow}`).values;
inputs.getRange(`D${baseEligibleRow}:H${baseEligibleRow}`).values=[[0,0,0,0,0]];wb.recalculate();
if(summary.getRange('H12').values[0][0]!==0)throw Error('Free-tier boundary failed');
inputs.getRange(`D${baseEligibleRow}:H${baseEligibleRow}`).values=original;
const churnRow=active.monthlyChurn+2,churnOriginal=inputs.getRange(`D${churnRow}:H${churnRow}`).values;
inputs.getRange(`D${churnRow}:H${churnRow}`).values=[[0,0,0,0,0]];wb.recalculate();
if(Math.abs(summary.getRange('H8').values[0][0]-280)>1e-8)throw Error('Business roll-forward failed');
inputs.getRange(`D${churnRow}:H${churnRow}`).values=churnOriginal;wb.recalculate();
if(Math.abs(summary.getRange('H17').values[0][0]-baseRevenue)>1e-8)throw Error('Input restoration failed');
const expected=JSON.parse(await fs.readFile(path.join(root,'scenario-results.json'),'utf8'));
for(const [name,values] of Object.entries(snapshots))for(let y=0;y<5;y++)for(const [r,k] of [[8,'end'],[9,'paidAverage'],[17,'revenue'],[23,'operating'],[27,'cash'],[29,'funding']]){
 if(Math.abs(values[r][y]-expected[name].years[y][k])>0.001)throw Error(`${name} ${y} ${k} mismatch`);
}
const errors=await wb.inspect({kind:'match',searchTerm:'#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A|#NUM!|#NULL!',options:{useRegex:true,maxResults:20},maxChars:1000});console.log(errors.ndjson);
console.log((await wb.inspect({kind:'table',range:'Summary!C17:H29',include:'values,formulas',tableMaxRows:13,tableMaxCols:6,maxChars:2000})).ndjson);
const qa=process.env.RESTRINGING_QA_DIR||'/tmp/restringing-business-qa';await fs.mkdir(qa,{recursive:true});
for(const [sheet,range] of [['Summary','C2:H37'],['Assumptions','C2:J24'],['Assumptions','C43:H64'],['Monthly build','C4:L16'],['Monthly build','M6:AE16'],['Assumptions','C83:H106'],['Case comparison','C2:H33']]){
 const img=await wb.render({sheetName:sheet,range,scale:1.5,format:'png'});await fs.writeFile(path.join(qa,sheet.replaceAll(' ','-')+'-'+range.replaceAll(':','-')+'.png'),new Uint8Array(await img.arrayBuffer()));
}
await (await SpreadsheetFile.exportXlsx(wb)).save(path.join(root,'financial-model.xlsx'));
await fs.writeFile(path.join(root,'workbook-results.json'),JSON.stringify(snapshots,null,2)+'\n');
await fs.rm(path.join(root,'financial-model.xlsx.inspect.ndjson'),{force:true});
console.log('Validated cases, price response, zero-paid boundary, churn roll-forward and restored base; exported financial-model.xlsx');
