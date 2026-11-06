#import "ContactsHistory.h"

@implementation ContactsHistory

+ (instancetype)since:(NSData *)token in:(CNContactStore *)store keys:(NSArray<id<CNKeyDescriptor>> *)keys error:(NSError **)error {
    CNChangeHistoryFetchRequest *req = [CNChangeHistoryFetchRequest new];
    req.startingToken = token;
    req.additionalContactKeyDescriptors = keys;
    req.shouldUnifyResults = YES;
    CNFetchResult<NSEnumerator<CNChangeHistoryEvent *> *> *res = [store enumeratorForChangeHistoryFetchRequest:req error:error];
    if (!res) {
        return nil;
    }
    ContactsHistory *h = [ContactsHistory new];
    h->_events = res.value.allObjects;
    h->_token = res.currentHistoryToken;
    return h;
}

@end
