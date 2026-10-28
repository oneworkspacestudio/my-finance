/** Production adapters are Actual's pinned native implementations, rather than
 * duplicate HTTP/ledger implementations. See docs/UPSTREAM_ACTUAL.md. */
export type ProviderId = 'enable-banking' | 'lunchflow-simplefin';
export interface BankAdapterCapabilities {
 id: ProviderId;
 nativeRoute: '/enablebanking' | '/simplefin';
 readOnly: true;
 ownerOnly: true;
 country: 'GB';
 experimental: boolean;
 requiresUserProviderAccount: true;
 unattendedSync: false;
}
export const adapters: readonly BankAdapterCapabilities[] = [
 {id:'enable-banking',nativeRoute:'/enablebanking',readOnly:true,ownerOnly:true,country:'GB',experimental:true,requiresUserProviderAccount:true,unattendedSync:false},
 {id:'lunchflow-simplefin',nativeRoute:'/simplefin',readOnly:true,ownerOnly:true,country:'GB',experimental:false,requiresUserProviderAccount:true,unattendedSync:false},
];
