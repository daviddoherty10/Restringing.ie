import json
from pathlib import Path
p=Path(__file__).resolve().parent; a=json.loads((p/'assumptions.json').read_text()); out={}
for s in a['scenarios']:
 active=0; cumulative=0; trough=0; rows=[]; years=[]; oldsites=0
 for y in range(5):
  annual=dict(revenue=0,subscription=0,events=0,pro=0,partners=0,lockers=0,variable=0,overhead=0,marketing=0,people=0,capex=0,startup=0,operating=0,cash=0,paidTotal=0,bookings=0)
  for m in range(12):
   adds=s['grossAdds'][y]/12; losses=active*s['monthlyChurn']; end=active+adds-losses; avg=(active+end)/2
   paid=avg*s['eligibleShare']*s['upgradeShare']*a['seasonality'][m]; books=avg*s['jobsPerBusiness']
   gross=[paid*a['prices']['subscription'],s['events'][y]/12*a['prices']['event'],s['proAccounts'][y]*a['prices']['pro'],s['partners'][y]/12*a['prices']['partnerAnnual'],s['lockerSites'][y]*s['lockerJobsPerSite']*a['prices']['lockerJob']]
   net=[v/(1+a['costs']['vatReserve']) for v in gross]
   transactions=paid+s['events'][y]/12+s['proAccounts'][y]+s['partners'][y]/12+s['lockerSites'][y]*s['lockerJobsPerSite']
   variable=sum(gross)*a['costs']['processorPercent']+transactions*a['costs']['processorFixed']+books*a['costs']['bookingVariable']+s['events'][y]/12*a['costs']['eventDelivery']+s['proAccounts'][y]*a['costs']['proAccountMonthly']+s['lockerSites'][y]*s['lockerJobsPerSite']*a['costs']['lockerJobVariable']+s['lockerSites'][y]*a['costs']['lockerSiteMonthly']
   op=sum(net)-variable-s['monthlyOverhead'][y]-s['annualMarketing'][y]/12-s['annualPeople'][y]/12
   capex=max(0,s['lockerSites'][y]-oldsites)*a['costs']['lockerCapex'] if m==0 else 0
   startup=a['costs']['startup']*(1+a['costs']['startupContingency']) if y==m==0 else 0
   cash=op-capex-startup; cumulative+=cash; trough=min(trough,cumulative)
   r=dict(revenue=sum(net),subscription=net[0],events=net[1],pro=net[2],partners=net[3],lockers=net[4],variable=variable,overhead=s['monthlyOverhead'][y],marketing=s['annualMarketing'][y]/12,people=s['annualPeople'][y]/12,capex=capex,startup=startup,operating=op,cash=cash,paidTotal=paid,bookings=books)
   for k in annual: annual[k]+=r[k]
   rows.append(dict(year=2027+y,month=m+1,end=end,paid=paid,cumulative=cumulative,**r)); active=end
  oldsites=s['lockerSites'][y]; annual.update(year=2027+y,end=active,paidAverage=annual['paidTotal']/12,cumulative=cumulative,funding=-trough+a['costs']['cashBuffer']); years.append(annual)
 out[s['name']]=dict(years=years)
(p/'scenario-results.json').write_text(json.dumps(out,indent=2)+'\n')
for name,d in out.items():
 print(name)
 for y in d['years']: print(y['year'],{k:round(y[k]) for k in ['end','paidAverage','revenue','operating','cash','cumulative','funding']})
