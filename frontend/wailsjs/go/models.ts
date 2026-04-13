export namespace app {
	
	export class C2CItemDetailVO {
	    c2cItemsId: number;
	    skuId: number;
	    price: number;
	    showPrice: string;
	    sellerName: string;
	    sellerUID: string;
	    publishTime: number;
	    status: string;
	    link: string;
	
	    static createFrom(source: any = {}) {
	        return new C2CItemDetailVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.c2cItemsId = source["c2cItemsId"];
	        this.skuId = source["skuId"];
	        this.price = source["price"];
	        this.showPrice = source["showPrice"];
	        this.sellerName = source["sellerName"];
	        this.sellerUID = source["sellerUID"];
	        this.publishTime = source["publishTime"];
	        this.status = source["status"];
	        this.link = source["link"];
	    }
	}
	export class C2CItemDetailListVO {
	    skuId: number;
	    c2cItemsName: string;
	    detailImg: string;
	    items: C2CItemDetailVO[];
	    total: number;
	    totalPages: number;
	    currentPage: number;
	
	    static createFrom(source: any = {}) {
	        return new C2CItemDetailListVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skuId = source["skuId"];
	        this.c2cItemsName = source["c2cItemsName"];
	        this.detailImg = source["detailImg"];
	        this.items = this.convertValues(source["items"], C2CItemDetailVO);
	        this.total = source["total"];
	        this.totalPages = source["totalPages"];
	        this.currentPage = source["currentPage"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class C2CItemGroupVO {
	    skuId: number;
	    c2cItemsName: string;
	    detailImg: string;
	    itemCount: number;
	    latestPublishTime: number;
	
	    static createFrom(source: any = {}) {
	        return new C2CItemGroupVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skuId = source["skuId"];
	        this.c2cItemsName = source["c2cItemsName"];
	        this.detailImg = source["detailImg"];
	        this.itemCount = source["itemCount"];
	        this.latestPublishTime = source["latestPublishTime"];
	    }
	}
	export class C2CItemGroupListVO {
	    items: C2CItemGroupVO[];
	    total: number;
	    totalPages: number;
	    currentPage: number;
	
	    static createFrom(source: any = {}) {
	        return new C2CItemGroupListVO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], C2CItemGroupVO);
	        this.total = source["total"];
	        this.totalPages = source["totalPages"];
	        this.currentPage = source["currentPage"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LoginInfo {
	    key: string;
	    login_url: string;
	
	    static createFrom(source: any = {}) {
	        return new LoginInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.login_url = source["login_url"];
	    }
	}
	export class MarketFilterOption {
	    label: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new MarketFilterOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.value = source["value"];
	    }
	}
	export class MarketRuntimeConfig {
	    categories: MarketFilterOption[];
	    sorts: MarketFilterOption[];
	    priceFilters: MarketFilterOption[];
	    discountFilters: MarketFilterOption[];
	    source: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new MarketRuntimeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.categories = this.convertValues(source["categories"], MarketFilterOption);
	        this.sorts = this.convertValues(source["sorts"], MarketFilterOption);
	        this.priceFilters = this.convertValues(source["priceFilters"], MarketFilterOption);
	        this.discountFilters = this.convertValues(source["discountFilters"], MarketFilterOption);
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class VerifyLoginResponse {
	    status: string;
	    cookies: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new VerifyLoginResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.cookies = source["cookies"];
	        this.message = source["message"];
	    }
	}

}

export namespace dao {
	
	export class ScrapyItem {
	    id: number;
	    priceFilter: string;
	    priceFilterLabel: string;
	    discountFilter: string;
	    discountFilterLabel: string;
	    product: string;
	    productName: string;
	    nums: number;
	    order: string;
	    increaseNumber: number;
	    nextToken?: string;
	    // Go type: time
	    createTime: any;
	
	    static createFrom(source: any = {}) {
	        return new ScrapyItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.priceFilter = source["priceFilter"];
	        this.priceFilterLabel = source["priceFilterLabel"];
	        this.discountFilter = source["discountFilter"];
	        this.discountFilterLabel = source["discountFilterLabel"];
	        this.product = source["product"];
	        this.productName = source["productName"];
	        this.nums = source["nums"];
	        this.order = source["order"];
	        this.increaseNumber = source["increaseNumber"];
	        this.nextToken = source["nextToken"];
	        this.createTime = this.convertValues(source["createTime"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

