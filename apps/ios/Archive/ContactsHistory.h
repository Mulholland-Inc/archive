#import <Contacts/Contacts.h>

NS_ASSUME_NONNULL_BEGIN

// Contacts' change history, which Swift cannot enumerate (NS_SWIFT_UNAVAILABLE): the events since a token, and the token after them.
// A nil token gives a drop-everything event and then every contact as added.
@interface ContactsHistory : NSObject
@property (nonatomic, readonly) NSArray<CNChangeHistoryEvent *> *events;
@property (nonatomic, readonly, nullable) NSData *token;
+ (nullable instancetype)since:(nullable NSData *)token in:(CNContactStore *)store keys:(NSArray<id<CNKeyDescriptor>> *)keys error:(NSError **)error;
@end

NS_ASSUME_NONNULL_END
